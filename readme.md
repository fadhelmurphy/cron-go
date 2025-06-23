# A Job Scheduler tool
inspired by cron and built with Golang

## How to build

### Windows
```bash
go build -o cron-go.exe
```

### Unix
```bash
go build -o cron-go
```

## Run multiple cron jobs with yaml 

### YAML example

```yaml
jobs:
  - schedule: "*/1 * * * *"
    command: "echo Job1: runs every minute"

  - schedule: "0,30 * * * *"
    command: "echo Job2: runs at 0 and 30 min of every hour"

  - schedule: "15-45 * * * *"
    command: "echo Job3: runs at min 15 to 45 of every hour"

```

### Run

use -d to run detach/background

```bash
cron-go -d jobs.yaml
```

## Run simple jobs

```bash
./cron-go -d "*/1 * * * * echo Menjalankan 1 menit"
```

## Stop the cron job

```bash
./cron-go stop <pid>
```

example:

```bash
./cron-go stop 2900
```