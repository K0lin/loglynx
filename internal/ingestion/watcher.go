package ingestion

import (
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/pterm/pterm"
)

// FileWatcher monitors log files for changes using fsnotify
type FileWatcher struct {
	watcher *fsnotify.Watcher
	paths   []string
	events  chan string // Channel for file modification events
	errors  chan error
	logger  *pterm.Logger
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewFileWatcher creates a new file watcher for the specified paths
func NewFileWatcher(paths []string, logger *pterm.Logger) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.WithCaller().Error("Failed to create file watcher", logger.Args("error", err))
		return nil, err
	}

	fw := &FileWatcher{
		watcher: watcher,
		paths:   paths,
		events:  make(chan string, 100),
		errors:  make(chan error, 10),
		logger:  logger,
		stopCh:  make(chan struct{}),
	}

	// Add all paths to watch
	successCount := 0
	for _, path := range paths {
		// Check if file exists before trying to watch
		if _, err := os.Stat(path); os.IsNotExist(err) {
			logger.Warn("Log file does not exist yet, skipping watch (will retry on file creation)", 
				logger.Args("path", path))
			continue
		}
		
		// Check if we have permission to read the file
		if file, err := os.Open(path); err != nil {
			if os.IsPermission(err) {
				logger.Error("Permission denied for log file", 
					logger.Args("path", path, "error", err))
			} else {
				logger.Warn("Cannot access log file, skipping watch", 
					logger.Args("path", path, "error", err))
			}
			continue
		} else {
			file.Close()
		}
		
		if err := watcher.Add(path); err != nil {
			logger.Warn("Failed to watch file", logger.Args("path", path, "error", err))
			continue
		}
		logger.Debug("Started watching file", logger.Args("path", path))
		successCount++
	}

	if successCount == 0 {
		logger.Warn("No log files are currently available to watch. Will continue running and watch for new files.")
	}

	// Start event loop
	fw.wg.Add(1)
	go fw.eventLoop()

	logger.Info("File watcher initialized", 
		logger.Args("files_watched", successCount, "files_pending", len(paths)-successCount))
	return fw, nil
}

// eventLoop processes file system events
func (fw *FileWatcher) eventLoop() {
	defer fw.wg.Done()

	for {
		select {
		case <-fw.stopCh:
			fw.logger.Debug("File watcher stopped")
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				fw.logger.Warn("File watcher events channel closed")
				return
			}

			// Handle different event types
			switch {
			case event.Op&fsnotify.Write == fsnotify.Write:
				fw.logger.Trace("File write detected", fw.logger.Args("file", event.Name))
				select {
				case fw.events <- event.Name:
				default:
					fw.logger.Warn("Event channel full, dropping event", fw.logger.Args("file", event.Name))
				}

			case event.Op&fsnotify.Create == fsnotify.Create:
				fw.logger.Debug("File created", fw.logger.Args("file", event.Name))
				// Re-add watch in case of log rotation
				if err := fw.watcher.Add(event.Name); err != nil {
					fw.logger.WithCaller().Warn("Failed to watch new file", fw.logger.Args("file", event.Name, "error", err))
				}

			case event.Op&fsnotify.Remove == fsnotify.Remove:
				fw.logger.Debug("File removed (possible rotation)", fw.logger.Args("file", event.Name))
				select {
				case fw.events <- event.Name:
				default:
					fw.logger.Warn("Event channel full, dropping remove event", fw.logger.Args("file", event.Name))
				}

			case event.Op&fsnotify.Rename == fsnotify.Rename:
				fw.logger.Debug("File renamed (possible rotation)", fw.logger.Args("file", event.Name))
				select {
				case fw.events <- event.Name:
				default:
					fw.logger.Warn("Event channel full, dropping rename event", fw.logger.Args("file", event.Name))
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				fw.logger.Warn("File watcher errors channel closed")
				return
			}
			fw.logger.WithCaller().Error("File watcher error", fw.logger.Args("error", err))
			select {
			case fw.errors <- err:
			default:
				fw.logger.Warn("Error channel full, dropping error")
			}
		}
	}
}

// Events returns the channel for file modification events
func (fw *FileWatcher) Events() <-chan string {
	return fw.events
}

// Errors returns the channel for watcher errors
func (fw *FileWatcher) Errors() <-chan error {
	return fw.errors
}

// AddPath adds a new path to watch
func (fw *FileWatcher) AddPath(path string) error {
	if err := fw.watcher.Add(path); err != nil {
		fw.logger.WithCaller().Warn("Failed to add watch path", fw.logger.Args("path", path, "error", err))
		return err
	}
	fw.paths = append(fw.paths, path)
	fw.logger.Info("Added new watch path", fw.logger.Args("path", path))
	return nil
}

// RemovePath removes a path from watching
func (fw *FileWatcher) RemovePath(path string) error {
	if err := fw.watcher.Remove(path); err != nil {
		fw.logger.WithCaller().Warn("Failed to remove watch path", fw.logger.Args("path", path, "error", err))
		return err
	}
	fw.logger.Info("Removed watch path", fw.logger.Args("path", path))
	return nil
}

// Close stops the file watcher and cleans up resources
func (fw *FileWatcher) Close() error {
	fw.logger.Debug("Closing file watcher...")
	close(fw.stopCh)
	fw.wg.Wait()

	if err := fw.watcher.Close(); err != nil {
		fw.logger.WithCaller().Error("Failed to close file watcher", fw.logger.Args("error", err))
		return err
	}

	close(fw.events)
	close(fw.errors)
	fw.logger.Info("File watcher closed")
	return nil
}
