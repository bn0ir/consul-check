package main


import (
	"fmt"
	"os"
	"log"
	"time"
	"strconv"
	"strings"
	"syscall"
	"os/exec"
	"os/signal"
	"encoding/json"
)
import "github.com/armon/consul-api"


type Operation struct {
	Key string
	Script string
	Interval int
	Timeout int
}

type Config struct {
	PidFile string
	LogFile string
	Consul consulapi.Config
	Operations []Operation
}

type Flags struct {
	stop bool
	reload bool
}

type Check struct {
	operation Operation
	update int64
}


func check(e error) {
	if e != nil {
		panic(e)
	}
}

func loadVars(filename string) Config {
	f, err := os.Open(filename)
	check(err)
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()
	ops := Config{}
	errOps := json.NewDecoder(f).Decode(&ops)
	check(errOps)
	return ops
}

func loadChecks(config Config) []Check {
	checks := make([]Check, 0)
	for _, element := range config.Operations {
		tempcheck := Check{operation: element, update: time.Now().Unix()}
		checks = append(checks, tempcheck)
	}
	return checks
}

func writePid(pidfile string) {
	mypid := os.Getpid()
	bytepid := []byte(strconv.Itoa(mypid))
	f, err := os.Create(pidfile)
	check(err)
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()
	if _, err := f.Write(bytepid); err != nil {
		panic(err)
	}
	return
}

func setLog(logfile string, logf *os.File) *os.File {
	if logf==(&os.File{}) {
		logf.Close()
	}
	if logfile=="" {
		return os.Stdout
	}
	logf, errLog := os.OpenFile(logfile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0640)
	if errLog != nil {
		fmt.Printf("Error opening file: %v", errLog)
	}
	return logf
}

func checkConsul(consul *consulapi.Config) {
	client, errCli := consulapi.NewClient(consul)
	check(errCli)
	_, errLea := client.Status().Leader()
	check(errLea)
	return
}

func checkSignal(sigcall chan os.Signal, message chan string) {
	for {
		s := <-sigcall
		switch s {
			case syscall.SIGHUP:
				message <- "reload"
			default:
				message <- "stop"
		}
	}
	return
}

func checkService(client *consulapi.Client, operation Operation) bool {
	queryoptions := consulapi.QueryOptions{AllowStale: true, RequireConsistent: false}
	checks, _, _ := client.Health().Checks(operation.Key, &queryoptions)
	status := true
	if len(checks) == 0 {
		log.Printf("Service %s does not exist\n", operation.Key)
		return status
	}
	for _, element := range checks {
		tempstatus := false
		if element.Status == "passing" {
			tempstatus = true
		}
		status = status && tempstatus
	}
	return status
}

func runProcess(cmd *exec.Cmd, name string) {
	data, err := cmd.Output()
	if err != nil {
		log.Printf("%s output: %s\n", name, err)
	} else {
		log.Printf("%s output: %s\n", name, string(data))
	}
	return
}

func reloadService(operation Check) Check {
	data := strings.Split(operation.operation.Script, " ")
	datal := len(data)
	command := data[0]
	args := make([]string, 0)
	if datal>1 {
		args = data[1:datal]
	}
	go runProcess(exec.Command(command, args...), operation.operation.Key)
	operation.update = time.Now().Unix()
	return operation
}

func main() {
	//check config
	configfile := "./config.json"
	if _, errConf := os.Stat(configfile); os.IsNotExist(errConf) {
		fmt.Printf("No such file or directory: %s", configfile)
		return
	}

	//application flags
	flags := Flags{stop: false, reload: true}

	//application config
	config := Config{}

	//start signals checker
	sigc := make(chan os.Signal, 2)
	sigmessage := make(chan string, 2)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go checkSignal(sigc, sigmessage)

	//counter
	counter := 0

	//checks
	checks := make([]Check, 0)

	//log file
	logf := &os.File{}

	//main cycle
	for {
		//main cycle tick
		time.Sleep(time.Second * 1)
		counter++
		//check signals
		select {
			case message := <-sigmessage:
				switch message {
					case "reload":
						flags.reload = true
					case "stop":
						flags.stop = true
				}
			default:
				//empty waiting cycle
		}
		//need to reload
		if flags.reload {
			config = loadVars(configfile)
			checks = loadChecks(config)
			writePid(config.PidFile)
			//open log file
			logf = setLog(config.LogFile, logf)
			defer logf.Close()
			//reload log
			log.SetOutput(logf)
			checkConsul(&config.Consul)
			flags.reload = false
			log.Printf("%#v\n", checks)
			log.Printf("Reload config\n")
		}
		//need to stop
		if flags.stop {
			log.Printf("Stop process\n")
			break
		}
		currenttime := time.Now().Unix()
		consulconnect, errCon := consulapi.NewClient(&config.Consul)
		if  errCon!=nil {
			log.Printf("Can't connect to consul server\n")
			continue
		}
		
		for index, element := range checks {
			if (currenttime - int64(element.operation.Timeout)) <= element.update {
				continue
			}
			if (currenttime - int64(element.operation.Interval)) <= element.update {
				continue
			}
			//check service status
			if !checkService(consulconnect, element.operation) {
				checks[index] = reloadService(element)
				log.Printf("%s: %s\n", element.operation.Key, element.operation.Script)
			}
		}

		//drop counter every day
		if counter >= 86400 {
			counter = 0
		}
	}
}
