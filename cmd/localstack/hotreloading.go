package main

import (
	"os"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/cmd/localstack/filenotify"
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

type ChangeListener struct {
	watcher            filenotify.FileWatcher
	changeChannel      chan string
	debouncedChannel   chan bool
	debouncingInterval time.Duration
	watchedFolders     []string
}

func NewChangeListener(debouncingInterval time.Duration, fileWatcherStrategy string) (*ChangeListener, error) {
	watcher, err := filenotify.New(200*time.Millisecond, fileWatcherStrategy)
	if err != nil {
		log.Errorln("Cannot create change listener due to filewatcher error.", err)
		return nil, err
	}
	changeListener := &ChangeListener{
		changeChannel:      make(chan string, 10),
		debouncedChannel:   make(chan bool, 10),
		debouncingInterval: debouncingInterval,
		watcher:            watcher,
	}
	return changeListener, nil
}

func (c *ChangeListener) Start() {
	c.debounceChannel()
	c.Watch()
}

func (c *ChangeListener) Watch() {
	for {
		select {
		case event, ok := <-c.watcher.Events():
			if !ok {
				close(c.changeChannel)
				return
			}
			log.Debugln("FileWatcher got event: ", event)
			if event.Has(fsnotify.Create) {
				stat, err := os.Stat(event.Name)
				if err != nil {
					log.Errorln("Error stating event file: ", event.Name, err)
				} else if stat.IsDir() {
					subfolders := getSubFolders(event.Name)
					for _, folder := range subfolders {
						err = c.watcher.Add(folder)
						c.watchedFolders = append(c.watchedFolders, folder)
						if err != nil {
							log.Errorln("Error watching folder: ", folder, err)
						}
					}
				}
				// remove in case of remove / rename (rename within the folder will trigger a separate create event)
			} else if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				// remove all file watchers if it is in our folders list
				toBeRemovedDirs, newWatchedFolders := getSubFoldersInList(event.Name, c.watchedFolders)
				c.watchedFolders = newWatchedFolders
				for _, dir := range toBeRemovedDirs {
					err := c.watcher.Remove(dir)
					if err != nil {
						log.Warnln("Error removing path: ", event.Name, err)
					}
				}
			}
			c.changeChannel <- event.Name
		case err, ok := <-c.watcher.Errors():
			if !ok {
				log.Println("error:", err)
				return
			}
			log.Println("error:", err)
		}
	}
}

func (c *ChangeListener) AddTargetPaths(targetPaths []string) {
	// Add all target paths and subfolders
	for _, targetPath := range targetPaths {
		subfolders := getSubFolders(targetPath)
		log.Infoln("Subfolders: ", subfolders)
		for _, target := range subfolders {
			err := c.watcher.Add(target)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func (c *ChangeListener) debounceChannel() {
	// debouncer to limit restarts
	timer := time.NewTimer(c.debouncingInterval)
	// immediately stop the timer, since we do not want to reload right at the startup
	if !timer.Stop() {
		// we have to drain the channel in case the timer already fired
		<-timer.C
	}
	go func() {
		for {
			select {
			case _, more := <-c.changeChannel:
				if !more {
					timer.Stop()
					close(c.debouncedChannel)
					return
				}
				timer.Reset(c.debouncingInterval)
			case <-timer.C:
				c.debouncedChannel <- true
			}
		}
	}()
}

func (c *ChangeListener) Close() error {
	return c.watcher.Close()
}
