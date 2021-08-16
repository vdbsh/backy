# 📦 backy: tiny multiprocessing utility for file backups

## Features
* Directories synchronization 
* Full directories archiving: hourly, daily, weekly, monthly or yearly with auto re-archivation if archive lost or corrupted
* Using native rsync(rsync over SSH supported) and tar(+bzip2) tools from your OS
* No third-party dependencies
* Linux, macOS and *BSD supported

## Usage
```backy <task.json>```

## Task Configuration Format
```
{
	"destination": "~/Backup",
	"archiving_cycle": "monthly",
	"multiprocessing": true,
	"verbose_log": false,
	
	"directories_to_sync": [
		"~/Desktop",
		"~/Documents"
	],
	
	"directories_to_archive": [
		"~/Desktop",
		"~/Documents"
	],

	"exclude": [
		".*"
	]
}
```

## Scheduling
See ```scripts```   

**Linux**  
* https://en.wikipedia.org/wiki/Cron

**MacOS**  
* https://en.wikipedia.org/wiki/Launchd
* https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/ScheduledJobs.html#//apple_ref/doc/uid/10000172i-CH1-SW2

## 3-2-1 Backup Scheme Example
```
        1                2                         3
[Primary Directory]->[External Drive]->[Cloud Storage]
        |                ^        |                ^
        |                |        |                |
        [<=backy========>]        [<=Cloud App>===>]
```

**For a complex backup schemes it is highly recommended to split tasks to separate configuration files and time periods. Another tip is to chain your backup actions by backy exit codes.**

## Exit Codes
* 0 - Synchronization and archiving completed successfully
* 1 - No task configuration provided
* 2 - Can't read provided task configuration
* 3 - Synchronization completed with errors
* 4 - Archiving completed with errors
* 5 - Synchronization and archiving completed with errors

## Building
```go build backy.go```
