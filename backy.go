package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

const version string = "1.2"

type Task struct {
	VerboseLog           bool     `json:"verbose_log"`
	Multiprocessing      bool     `json:"multiprocessing"`
	Destination          string   `json:"destination"`
	ArchivingCycle       string   `json:"archiving_cycle"`
	Exclude              []string `json:"exclude"`
	DirectoriesToSync    []string `json:"directories_to_sync"`
	DirectoriesToArchive []string `json:"directories_to_archive"`
}

func removeDuplicates(elements []string) (unique_elements []string) {
	encountered := map[string]bool{}

	for i := range elements {
		if encountered[elements[i]] == true {
		} else {
			encountered[elements[i]] = true
			unique_elements = append(unique_elements, elements[i])
		}
	}
	return unique_elements
}

func normilizePaths(paths []string) (normalized_paths []string) {
	homedir, _ := os.UserHomeDir()

	for i, p := range paths {
		if string(p[0]) == "~" {
			p = strings.Replace(p, "~", homedir, 1)
		}
		paths[i] = strings.TrimRight(string(p), "/")
	}
	return paths
}

func getBasePath(path string) (base_path string) {
	hostname, err := os.Hostname()

	if err != nil {
		hostname = "local"
	}
	return strings.Replace(hostname+path, "/", "_", -1)
}

func formatExcludeArgs(exclude_args_list []string) (exclude_args []string) {
	for _, i := range exclude_args_list {
		exclude_args = append(exclude_args, "--exclude="+i)
	}
	return exclude_args
}

func generateArchiveFilePath(path string, archiving_cycle string) (file_path string) {
	var result_file_path [1]string

	dt := time.Now()
	_, iso_week := dt.ISOWeek()
	base_path := getBasePath(path)

	switch archiving_cycle {
	case "hourly":
		result_file_path[0] = base_path + dt.Format("_2006_January_") + strconv.Itoa(dt.Day()) + dt.Format("_15hr") + ".tar.bz2"
	case "daily":
		result_file_path[0] = base_path + dt.Format("_2006_January_") + strconv.Itoa(dt.Day()) + ".tar.bz2"
	case "weekly":
		result_file_path[0] = base_path + dt.Format("_2006_January_") + strconv.Itoa(iso_week) + "wk.tar.bz2"
	case "monthly":
		result_file_path[0] = base_path + dt.Format("_2006_January") + ".tar.bz2"
	case "yearly":
		result_file_path[0] = base_path + dt.Format("_2006") + ".tar.bz2"
	default:
		result_file_path[0] = base_path + dt.Format("_2006_January") + ".tar.bz2"
	}

	return result_file_path[0]
}

func getTaskFromJson(file_path string) (config_err error, task_config Task) {
	jsonFile, err := os.Open(file_path)

	if err != nil {
		config_err = errors.New("Can't open task file")
	}

	log.Println("📋", "Working with", file_path, "...")
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	json.Unmarshal(byteValue, &task_config)

	if task_config.Destination != "" {
		task_config.Destination = normilizePaths([]string{task_config.Destination})[0]
		task_config.DirectoriesToSync = removeDuplicates(normilizePaths(task_config.DirectoriesToSync))
		task_config.DirectoriesToArchive = removeDuplicates(normilizePaths(task_config.DirectoriesToArchive))
		task_config.Exclude = formatExcludeArgs(removeDuplicates(task_config.Exclude))
	} else {
		log.Println("🔰", "Task configuration: destination must be specified!")
		config_err = errors.New("Wrong task format")
	}
	return config_err, task_config
}

func filterArchiveDirs(archive_dirs []string, archiving_cycle string, dest string) (filtered_archive_dirs []string) {
	for _, i := range archive_dirs {
		_, e := os.Stat(path.Join(dest, generateArchiveFilePath(i, archiving_cycle)))
		if os.IsNotExist(e) {
			filtered_archive_dirs = append(filtered_archive_dirs, i)
		}
	}
	return filtered_archive_dirs
}

func logProgress(amount int, indicator string) {
	progress_indicator_step := " " + indicator
	for i := 1; i < amount; i++ {
		indicator += progress_indicator_step
	}
	log.Println(indicator)
}

func checkStatus(status error, process string, element string) (status_err error) {
	if status != nil {
		log.Println("💢", process, "failed for:", element)
		status_err = errors.New("process failed")
	}
	return status_err
}

func setProcessOutput(process *exec.Cmd, print_outtput bool) {
	if print_outtput == true {
		process.Stdout = os.Stdout
		process.Stderr = os.Stderr
	}
}

func runProcess(c string, args []string, print_outtput bool) (status error) {
	cmd := exec.Command(c, args...)
	setProcessOutput(cmd, print_outtput)
	return cmd.Run()
}

func startProcess(c string, args []string, print_outtput bool) (handle *exec.Cmd) {
	cmd := exec.Command(c, args...)
	setProcessOutput(cmd, print_outtput)
	cmd.Start()
	return cmd
}

func startRsync(src_dirs []string, dest string, args []string, multiprocessing bool, print_outtput bool) (rsync_err error) {
	var processes []*exec.Cmd

	if len(src_dirs) > 0 {
		log.Println("📤", "Syncing from:", src_dirs)
		log.Println("📥", "To:", dest, "...")

		for p, i := range src_dirs {
			if multiprocessing == true {
				processes = append(processes, startProcess("rsync", append(args, []string{i, dest}...), print_outtput))
			} else {
				logProgress(p+1, "🗂")
				if checkStatus(runProcess("rsync", append(args, []string{i, dest}...), print_outtput), "Syncronization", i) != nil {
					rsync_err = errors.New("rsync failed")
				}
			}
		}
		if multiprocessing == true {
			for i, p := range processes {
				logProgress(i+1, "🗂")
				if checkStatus(p.Wait(), "Syncronization", src_dirs[i]) != nil {
					rsync_err = errors.New("rsync failed")
				}
			}
		}
	} else {
		log.Println("💤", "Nothing to sync")
	}
	return rsync_err
}

func startTar(archive_dirs []string, dest string, mode string, args []string, archiving_cycle string, multiprocessing bool, print_outtput bool) (tar_err error) {
	var processes []*exec.Cmd

	dirs_to_archive := filterArchiveDirs(archive_dirs, archiving_cycle, dest)
	if len(dirs_to_archive) > 0 {
		log.Println("📤", "Archiving:", dirs_to_archive)
		log.Println("📥", "To:", dest, "...")

		for p, i := range dirs_to_archive {
			completed_archive := path.Join(dest, generateArchiveFilePath(i, archiving_cycle))
			in_progress_archive := completed_archive + ".part"
			task_args := append([]string{mode, in_progress_archive}, append(args, i)...)

			_, e := os.Stat(in_progress_archive)
			if !os.IsNotExist(e) {
				os.Remove(in_progress_archive)
			}

			if multiprocessing == true {
				processes = append(processes, startProcess("tar", task_args, print_outtput))
			} else {
				logProgress(p+1, "📦")
				if checkStatus(runProcess("tar", task_args, print_outtput), "Archiving", i) != nil {
					os.Remove(in_progress_archive)
					tar_err = errors.New("tar failed")
				} else {
					os.Rename(in_progress_archive, completed_archive)
				}
			}
		}

		if multiprocessing == true {
			for i, p := range processes {
				completed_archive := path.Join(dest, generateArchiveFilePath(dirs_to_archive[i], archiving_cycle))
				in_progress_archive := completed_archive + ".part"
				logProgress(i+1, "📦")
				if checkStatus(p.Wait(), "Archiving", dirs_to_archive[i]) != nil {
					os.Remove(in_progress_archive)
					tar_err = errors.New("tar failed")
				} else {
					os.Rename(in_progress_archive, completed_archive)
				}
			}
		}

	} else {
		log.Println("💤", "Nothing to archive")
	}
	return tar_err
}

func main() {
	var status int = 0
	log.SetOutput(os.Stdout)
	log.Println("🍀", "backy", version)

	if len(os.Args[1:]) < 1 {
		log.Println("❗️", "No task configuration provided")
		log.Println("🔰", "Usage: backy <task.json>")
		status = 1

	} else {
		err, task := getTaskFromJson(os.Args[1])
		if err != nil {
			log.Println("❗️", "Can't read provided task configuration")
			status = 2

		} else {
			if startRsync(task.DirectoriesToSync, task.Destination, append([]string{"-avW", "--delete", "--delete-excluded"}, task.Exclude...), task.Multiprocessing, task.VerboseLog) != nil {
				status = 3
				log.Println("❗️", "Synchronization completed with errors")
			}

			if startTar(task.DirectoriesToArchive, task.Destination, "-jcvf", task.Exclude, task.ArchivingCycle, task.Multiprocessing, task.VerboseLog) != nil {
				if status != 3 {
					status = 4
				} else {
					status = 5
				}
				log.Println("❗️", "Archiving completed with errors")
			}
		}
	}
	os.Exit(status)
}
