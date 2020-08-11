// example script fmc_run.sh
// #!/bin/bash
// echo "file change... args...(  $1 $2 $3 $4 $5 )"
package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/howeyc/fsnotify"
)

type FmoncInfo struct {
	Fname         string `json:"fname"`
	FmoncPathFull string `json:fmoncpathfull`
	Fsize         int64  `json:"fsize"`
	Fchange       int8   `json:"fchange"`
	Md5sumFile    string `json:md5sumfile`
	Ftime         int64  `json:"ftime"`
}
type ConfigFilemonc struct {
	Filemonc FmoncSet
}
type FmoncSet struct {
	FmoncPath      []string
	FmoncRunScript string
	FmoncPathLog   string
	FmoncFormatLog string
	TypeMode       int16
}

var (
	cfg *ConfigFilemonc
)

func init() {
	cfg = ReadConfigNs()

}
func ReadConfigNs() (c *ConfigFilemonc) {
	file, _ := os.Open("filemonc.json")
	decoder := json.NewDecoder(file)
	CfgFilemonc := new(ConfigFilemonc)
	err := decoder.Decode(&CfgFilemonc)
	if err != nil {
		log.Fatalln("error parse config: ", err)
	}
	return CfgFilemonc
}
func writeLogScript(mytxt interface{}) {
	file, err := os.OpenFile(cfg.Filemonc.FmoncPathLog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Println("err: ", err)
		os.Exit(1)
	}
	defer file.Close()
	w := bufio.NewWriter(file)
	fmt.Fprintln(w, mytxt)
	w.Flush()
}
func execCmd(script_path string, fm_info FmoncInfo) error {
	args := []string{script_path, fm_info.Fname,
		strconv.Itoa(int(fm_info.Fsize)), strconv.Itoa(int(fm_info.Ftime)), fm_info.Md5sumFile}
	cmd := exec.Command("bash", args...)
	cmd.Stdin = strings.NewReader("")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		writeLogScript(time.Now().Format(cfg.Filemonc.FmoncFormatLog) + " script_run: " + script_path + " " + err.Error())
		return err
	}
	writeLogScript(time.Now().Format(cfg.Filemonc.FmoncFormatLog) + " " + out.String())
	return err
}

func md5sum(filePath string) (result string, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return
	}

	result = hex.EncodeToString(hash.Sum(nil))
	return
}
func FsEvents(base_path string, ev *fsnotify.FileEvent) FmoncInfo {
	//
	var fmoncinfo FmoncInfo
	var fStat int8
	path := base_path + ev.Name
	f, e := os.Stat(base_path)
	if e != nil {
	}
	if ev.IsCreate() {
		fStat = 0
	}
	if ev.IsDelete() {
		fStat = 1
	}
	if ev.IsModify() {
		fStat = 2
	}
	if ev.IsRename() {
		fStat = 3
	}
	Md5sumFile, err := md5sum(ev.Name)
	if err != nil {
		log.Println("error md5sum: ", err)
	}
	fmoncinfo = FmoncInfo{ev.Name, path, f.Size(), fStat, Md5sumFile, f.ModTime().UnixNano()}
	//debug
	// log.Println("debug : ", fmoncinfo)
	go execCmd(cfg.Filemonc.FmoncRunScript, fmoncinfo)
	return fmoncinfo

}
func MyMonitorFmoncStart() {
	var wg sync.WaitGroup
	//test dir
	for _, base_path := range cfg.Filemonc.FmoncPath {
		if _, err := os.Stat(base_path); os.IsNotExist(err) {
			log.Println("error dir: ", err)
			os.Exit(1)
		}
	}
	wg.Add(len(cfg.Filemonc.FmoncPath))
	for _, base_path := range cfg.Filemonc.FmoncPath {
		go func(base_path string) {
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				log.Fatal(err)
			}
			err = watcher.Watch(base_path)
			if err != nil {
				log.Fatal(err)
			} else {
				log.Println("mon dir: ", base_path, " ok...")
			}
			done := make(chan bool)
			// Process events
			go func() {
				for {
					select {
					case ev := <-watcher.Event:
						FsEvents(base_path, ev)
						if ev.IsCreate() {
							fi, err := os.Stat(ev.Name)
							if err != nil {
								log.Println("os.Stat: ", err)
							}
							switch mode := fi.Mode(); {
							case mode.IsDir():
								err = watcher.Watch(ev.Name)
								if err != nil {
									log.Println("error watch add new dir: ", err)
								}
							case mode.IsRegular():
								// do file
								//log.Println("file")
							}
						}
					case err := <-watcher.Error:
						log.Println("error:", err)
					}
				}
			}()
			<-done
			watcher.Close()
		}(base_path)
	}
	wg.Wait()
}
func main() {
	log.Println("start...")
	MyMonitorFmoncStart()
}
