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

const version string = "1.1"

type Task struct {
	VerboseLog           bool     `json:"verbose_log"`
	Multiprocessing      bool     `json:"multiprocessing"`
	RemoveFailedArchives bool     `json:"remove_failed_archives"`
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

	for i, path := range paths {
		if string(path[0]) == "~" {
			path = strings.Replace(path, "~", homedir, 1)
		}
		paths[i] = strings.TrimRight(string(path), "/")
	}
	return paths
}

func formatExcludeArgs(exclude_args_list []string) (exclude_args []string) {
	for _, el := range exclude_args_list {
		exclude_args = append(exclude_args, "--exclude="+el)
	}
	return exclude_args
}

func generateArchiveFilePath(path string, archiving_cycle string) (file_path string) {
	var result_file_path [1]string

	dt := time.Now()
	_, iso_week := dt.ISOWeek()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "local"
	}

	base_path := strings.Replace(hostname+path, "/", "_", -1)

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

func executeProcesses(c string, args [][]string, multiprocessing bool, progress_indicator string, print_outtput bool) (statuses []error) {
	var processes []*exec.Cmd
	progress_indicator_step := " " + progress_indicator

	for _, arg_set := range args {
		cmd := exec.Command(c, arg_set...)
		if print_outtput == true {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if multiprocessing == true {
			processes = append(processes, cmd)
			cmd.Start()
		} else {
			log.Println(progress_indicator)
			statuses = append(statuses, cmd.Run())
			progress_indicator += progress_indicator_step
		}
	}

	if multiprocessing == true {
		for _, process := range processes {
			log.Println(progress_indicator)
			statuses = append(statuses, process.Wait())
			if print_outtput == true {
				log.Println()
			}
			progress_indicator += progress_indicator_step
		}
	}
	return statuses
}

func rsync(src_dirs []string, dest string, args []string, multiprocessing bool, print_outtput bool) (rsync_err error) {
	if len(src_dirs) > 0 {
		var task_args [][]string

		log.Println("📤", "Syncing from:", src_dirs)
		log.Println("📥", "To:", dest, "...")

		for _, dir := range src_dirs {
			task_args = append(task_args, append(args, []string{dir, dest}...))
		}

		results := executeProcesses("rsync", task_args, multiprocessing, "🗂", print_outtput)

		for i, status := range results {
			if status != nil {
				log.Println("💢", "Syncronisation failed for:", src_dirs[i])
				rsync_err = errors.New("rsync failed")
			}
		}
	} else {
		log.Println("💤", "Nothing to sync")
	}
	return rsync_err
}

func tar(archive_dirs []string, dest string, mode string, args []string, archiving_cycle string, multiprocessing bool, print_outtput bool, remove_failed_archives bool) (tar_err error) {
	var archives []string
	var dirs_to_archive []string
	var task_args [][]string

	for _, dir := range archive_dirs {
		file_path := generateArchiveFilePath(dir, archiving_cycle)
		archives = append(archives, path.Join(dest, file_path))
	}

	for i, archive := range archives {
		in_progress_archive := archive + ".part"

		if _, err := os.Stat(archive); err != nil {
			if _, err := os.Stat(in_progress_archive); err == nil {
				os.Remove(in_progress_archive)
			}
			task_args = append(task_args, append([]string{mode, in_progress_archive}, append(args, archive_dirs[i])...))
			dirs_to_archive = append(dirs_to_archive, archive_dirs[i])
		}
	}

	if len(dirs_to_archive) > 0 {
		log.Println("📤", "Archiving:", dirs_to_archive)
		log.Println("📥", "To:", dest, "...")
	} else {
		log.Println("💤", "Nothing to archive")
	}

	results := executeProcesses("tar", task_args, multiprocessing, "📦", print_outtput)

	for i, status := range results {
		completed_archive := path.Join(dest, generateArchiveFilePath(dirs_to_archive[i], archiving_cycle))

		if status != nil {
			log.Println("💢", "Archiving failed for:", dirs_to_archive[i])
			if remove_failed_archives == true {
				log.Println("🧹", "Removing failed archive")
				os.Remove(completed_archive + ".part")
			}
			tar_err = errors.New("tar failed")
		} else {
			os.Rename(completed_archive+".part", completed_archive)
		}
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
			if rsync(task.DirectoriesToSync, task.Destination, append([]string{"-avW", "--delete", "--delete-excluded"}, task.Exclude...), task.Multiprocessing, task.VerboseLog) != nil {
				status = 3
				log.Println("❗️", "Synchronization completed with errors")
			}

			if tar(task.DirectoriesToArchive, task.Destination, "-jcvf", task.Exclude, task.ArchivingCycle, task.Multiprocessing, task.VerboseLog, task.RemoveFailedArchives) != nil {
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
