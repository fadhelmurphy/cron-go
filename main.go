package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"sync"

	"gopkg.in/yaml.v3"
)

type Job struct {
	Schedule string `yaml:"schedule"`
	Command  string `yaml:"command"`
}

type Config struct {
	Jobs []Job `yaml:"jobs"`
}

type ParsedJob struct {
	MinuteSet map[int]bool
	HourSet   map[int]bool
	Schedule  string
	Command   string
}

var logMutex sync.Mutex

func stopBackground(pidStr string) error {
	pidStr = strings.TrimSpace(pidStr)
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("invalid PID: %v", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %v", err)
	}

	err = proc.Kill()
	if err != nil {
		return fmt.Errorf("failed to kill process: %v", err)
	}

	return nil
}

func runSingleJob(jobStr string) {
	fields := strings.Fields(jobStr)
	if len(fields) < 6 {
		fmt.Println("Invalid job string, must contain cron + command")
		os.Exit(1)
	}

	schedule := strings.Join(fields[0:5], " ")
	command := strings.Join(fields[5:], " ")

	job := Job{
		Schedule: schedule,
		Command:  command,
	}

	pj, err := parseJob(job)
	if err != nil {
		fmt.Printf("Invalid job schedule: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running single job: %s %s\n", schedule, command)

	for {
		now := time.Now()
		min := now.Minute()
		hour := now.Hour()

		if pj.MinuteSet[min] && pj.HourSet[hour] {
			fmt.Printf("[%s] Running: %s %s\n", now.Format(time.RFC3339), pj.Schedule, pj.Command)
			runCommand(pj.Schedule, pj.Command)
		}

		time.Sleep(time.Second * 30)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println(`Usage:
			cron-go [-d] <config.yaml>
			cron-go [-d] "<cron_expr>" "<command>"
			cron-go [-d] "<cron_expr> <command>"   # combined
			cron-go stop "<pid>"
	`)
		os.Exit(1)
	}

	runAsDaemon := false
	args := os.Args[1:]
	i := 0

	if args[0] == "-d" {
		runAsDaemon = true
		i++
	}

	if args[i] == "stop" && len(args) > i+1 && args[i+1] != "" {
		if err := stopBackground(args[i+1]); err != nil {
			fmt.Printf("Failed to stop cron: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Cron stopped.")
		return
	}

	var jobExpr, jobCmd, cfgPath, combinedJob string

	remaining := args[i:]
	if len(remaining) >= 2 {
		jobExpr = remaining[0]
		jobCmd = strings.Join(remaining[1:], " ")
	} else if len(remaining) == 1 {
		if strings.HasSuffix(remaining[0], ".yaml") {
			cfgPath = remaining[0]
		} else {
			combinedJob = remaining[0]
		}
	} else {
		fmt.Println("Invalid arguments.")
		os.Exit(1)
	}

	if runAsDaemon {
		if jobExpr != "" && jobCmd != "" {
			jobStr := fmt.Sprintf("%s %s", jobExpr, jobCmd)
			launchBackground([]string{jobStr})
		} else if combinedJob != "" {
			launchBackground([]string{combinedJob})
		} else if cfgPath != "" {
			launchBackground([]string{cfgPath})
		} else {
			fmt.Println("Invalid background usage.")
			os.Exit(1)
		}
		return
	}

	if jobExpr != "" && jobCmd != "" {
		jobStr := fmt.Sprintf("%s %s", jobExpr, jobCmd)
		runSingleJob(jobStr)
	} else if combinedJob != "" {
		runSingleJob(combinedJob)
	} else if cfgPath != "" {
		runScheduler(cfgPath)
	} else {
		fmt.Println("Invalid foreground usage.")
		os.Exit(1)
	}
}

func runScheduler(cfgPath string) {
	config, err := loadConfig(cfgPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	var jobs []ParsedJob
	for _, job := range config.Jobs {
		pj, err := parseJob(job)
		if err != nil {
			fmt.Printf("Invalid job schedule: %v\n", err)
			continue
		}
		jobs = append(jobs, pj)
	}

	fmt.Printf("Loaded %d jobs.\n", len(jobs))

	for {
		now := time.Now()
		min := now.Minute()
		hour := now.Hour()

		for _, j := range jobs {
			if j.MinuteSet[min] && j.HourSet[hour] {
				fmt.Printf("[%s] [%s] Running: %s\n", now.Format(time.RFC3339), j.Schedule, j.Command)
				runCommand(j.Schedule, j.Command)
			}
		}

		next := now.Truncate(time.Minute).Add(time.Minute)
		time.Sleep(time.Until(next))
	}

}

func loadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func parseJob(job Job) (ParsedJob, error) {
	fields := strings.Fields(job.Schedule)
	if len(fields) < 5 {
		return ParsedJob{}, fmt.Errorf("invalid cron expression")
	}

	minSet, err := parseField(fields[0], 0, 59)
	if err != nil {
		return ParsedJob{}, fmt.Errorf("minute field: %v", err)
	}
	hourSet, err := parseField(fields[1], 0, 23)
	if err != nil {
		return ParsedJob{}, fmt.Errorf("hour field: %v", err)
	}

	return ParsedJob{
		MinuteSet: minSet,
		HourSet:   hourSet,
		Schedule: job.Schedule,
		Command:   job.Command,
	}, nil
}

func parseField(field string, min, max int) (map[int]bool, error) {
	result := make(map[int]bool)

	if field == "*" {
		for i := min; i <= max; i++ {
			result[i] = true
		}
		return result, nil
	}

	for _, part := range strings.Split(field, ",") {
		if strings.HasPrefix(part, "*/") {
			stepStr := strings.TrimPrefix(part, "*/")
			step, err := strconv.Atoi(stepStr)
			if err != nil {
				return nil, err
			}
			for i := min; i <= max; i++ {
				if i%step == 0 {
					result[i] = true
				}
			}
		} else if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err1 := strconv.Atoi(rangeParts[0])
			end, err2 := strconv.Atoi(rangeParts[1])
			if err1 != nil || err2 != nil || start > end {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			for i := start; i <= end; i++ {
				if i < min || i > max {
					return nil, fmt.Errorf("value %d out of bounds", i)
				}
				result[i] = true
			}
		} else {
			val, err := strconv.Atoi(part)
			if err != nil {
				return nil, err
			}
			if val < min || val > max {
				return nil, fmt.Errorf("value %d out of bounds", val)
			}
			result[val] = true
		}
	}

	return result, nil
}

func runCommand(scheduleStr string, cmdStr string) {
	go func() {
		logFile, err := os.OpenFile("cron-output.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("Failed to open log file: %v\n", err)
			return
		}
		defer logFile.Close()

		fields := strings.Fields(cmdStr)
		if len(fields) == 0 {
			logMutex.Lock()
			logFile.WriteString(fmt.Sprintf("[%s] Invalid command: empty\n", time.Now().Format(time.RFC3339)))
			logMutex.Unlock()
			return
		}

		cmdName := fields[0]
		cmdArgs := fields[1:]

		proc := exec.Command(cmdName, cmdArgs...)

		// Output command
		// proc.Stdout = logFile
		// proc.Stderr = logFile

		err = proc.Start()
		if err != nil {
			logMutex.Lock()
			logFile.WriteString(fmt.Sprintf("[%s] PID: %d Schedule: %s Command failed to start: %v\n",
				time.Now().Format(time.RFC3339), os.Getpid(), scheduleStr, err))
			logMutex.Unlock()
			return
		}

		// childPid := proc.Process.Pid
		pid := os.Getpid()

		logLine := fmt.Sprintf("[%s] PID: %d Schedule: %s Running: %s\n",
			time.Now().Format(time.RFC3339), pid, scheduleStr, cmdStr)

		logMutex.Lock()
		logFile.WriteString(logLine)
		logMutex.Unlock()
		fmt.Print(logLine)

		err = proc.Wait()
		if err != nil {
			logMutex.Lock()
			logFile.WriteString(fmt.Sprintf("[%s] PID: %d Command failed: %v\n",
				time.Now().Format(time.RFC3339), pid, err))
			logMutex.Unlock()
		}
	}()
}





