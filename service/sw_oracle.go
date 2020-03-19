package service

import (
	"github.com/gin-gonic/gin"
	"github.com/magiconair/properties"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync/atomic"
	"time"
)

var (
	server *gin.Engine
	env    map[string]*OracleConn
	config string
	lock   int32
)

type OracleConn struct {
	Host     string `properties:"hbec.commons.rdbs.zjzbjy.host"`
	Port     string `properties:"hbec.commons.rdbs.zjzbjy.port,default=1521"`
	Username string `properties:"hbec.commons.rdbs.zjzbjy.username"`
	Password string `properties:"hbec.commons.rdbs.zjzbjy.password"`
	//    D time.Duration `properties:"timeout,default=5s"`
	//    E time.Time     `properties:"expires,layout=2006-01-02,default=2015-01-01"`
}

type Config struct {
	Envs []Env `yaml:"envs"`
}

type Env struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Username string `yaml:"user"`
	Password string `yaml:"pass"`
}

func Run(addr string, file string) {
	loadConfig(file)
	register()
	server.Run(addr)
}

func loadConfig(file string) {
	config := getConf(file)
	if len(config.Envs) > 0 {
		env = make(map[string]*OracleConn)
		for _, v := range config.Envs {
			env[v.Name] = &OracleConn{Host: v.Host, Port: v.Port, Username: v.Username, Password: v.Password}
		}
	}
}

func getConf(file string) *Config {
	c := Config{}
	yamlFile, err := ioutil.ReadFile(file)
	if err == nil {
		yaml.Unmarshal(yamlFile, &c)
	}
	return &c
}

func oracle(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("run time panic: %v", err)
			c.JSON(200, "error, please retry or contact to administrator.")
		}
	}()
	filename := c.DefaultQuery("filename", "/hbec/configs/hbec-app.properties")
	to := c.Query("to")
	if env[to] != nil {
		// switch env
		// lock
		if atomic.CompareAndSwapInt32(&lock, 0, 1) {
			defer func() {
				// release lock
				atomic.StoreInt32(&lock, 0)
			}()
			time.Sleep(time.Duration(5) * time.Second)
			properties.ErrorHandler = properties.PanicHandler
			property := properties.MustLoadFile(filename, properties.UTF8)
			msg := switchover(to, filename, property)
			c.JSON(200, msg)
		} else {
			// running in process, denied
			c.JSON(200, "switch in process, wait a moment and retry please.")
		}
	} else {
		var envstr string
		for key := range env {
			envstr += ("[" + key + "]")
		}
		c.JSON(200, "param [to] is required, must be "+envstr+".")
	}
}

func status(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("panic in running /switchover/status time: %v", err)
			c.JSON(200, "error, please retry or contact to administrator.")
		}
	}()
	if result, err := execCmd("/bin/bash", "-c", "service trade-searcher status"); err != nil {
		c.JSON(200, "check status faild. "+err.Error())
	} else {
		filename := c.DefaultQuery("filename", "/hbec/configs/hbec-app.properties")
		properties.ErrorHandler = properties.PanicHandler
		property := properties.MustLoadFile(filename, properties.UTF8)
		currentHost := property.GetString("hbec.commons.rdbs.zjzbjy.host", "")
		c.JSON(200, gin.H{"status": result, "currentHost": currentHost})
	}
}

func switchover(to string, toFilename string, property *properties.Properties) (result string) {
	// modify properties
	toOraconn := *env[to]
	envType := reflect.TypeOf(&toOraconn).Elem()
	envValue := reflect.ValueOf(toOraconn)
	for i := 0; i < envType.NumField(); i++ {
		field := envType.Field(i)
		key := field.Tag.Get("properties")
		if end := strings.Index(key, ","); end >= 0 {
			key = key[0:end]
		}
		value := envValue.FieldByName(field.Name).String()
		property.SetValue(key, value)
		logFile, err := os.Create(toFilename)
		if err != nil {
			log.Panicln("open properties faild when write comment to the properties.", err.Error())
		} else {
			if _, err := property.WriteComment(logFile, "#", properties.UTF8); err != nil {
				log.Panicln("write comment to the properties faild.", err.Error())
			}
		}
	}
	log.Println("switch to", to, ", properties has been modified.")
	// restart app
	var err error
	if result, err = execCmd("/bin/bash", "-c", "service trade-searcher restart"); err != nil {
		log.Panicln("restart faild.", err.Error())
	}
	return result
}

func execCmd(command string, args ...string) (result string, err error) {
	cmd := exec.Command(command, args...)
	if bytes, err := cmd.Output(); err == nil {
		result = string(bytes)
	} else {
		log.Panicln("exec.Command faild.", err.Error())
	}
	return
}

func authentication() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("request", "clinet_request")
		c.Next()
	}
}

func register() {
	switchover := server.Group("/switchover")
	switchover.Use(authentication())
	{
		switchover.GET("/oracle", oracle)
		switchover.GET("/status", status)
	}
}

func init() {
	server = gin.New()
	server.Use(gin.Logger())
	server.Use(gin.Recovery())
	log.SetOutput(os.Stdout)
	env = map[string]*OracleConn{}
}
