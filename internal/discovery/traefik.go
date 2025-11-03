package discovery

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"
	"loglynx/internal/database/models"
	"strings"

	"github.com/pterm/pterm"
)

type TraefikDetector struct{
    logger *pterm.Logger
	configuredPath string
}

func NewTraefikDetector(logger *pterm.Logger) ServiceDetector {
    return &TraefikDetector{
        logger: logger,
		configuredPath: os.Getenv("TRAEFIK_LOG_PATH"),
    }
}

func (d *TraefikDetector) Name() string {
    return "traefik"
}

func (d *TraefikDetector) Detect() ([]*models.LogSource, error) {
    sources := []*models.LogSource{}
    d.logger.Trace("Detecting Traefik log sources...")

	// Build paths list - prioritize configured path
    paths := []string{}

	// Add configured path from environment variable if set
	if d.configuredPath != "" {
		paths = append(paths, d.configuredPath)
		d.logger.Debug("Using configured Traefik log path", d.logger.Args("path", d.configuredPath))
	}

	// Add standard relative paths as fallback
	paths = append(paths, "traefik/logs/access.log", "traefik/logs/error.log")

    for _, path := range paths {
        d.logger.Trace("Checking", d.logger.Args("path", path))
        if fileInfo, err := os.Stat(path); err == nil {
            d.logger.Trace("File found", d.logger.Args("path", path))
            if !fileInfo.IsDir() && fileInfo.Size() > 0 {
                d.logger.Trace("Validating format", d.logger.Args("path", path))
                if isTraefikFormat(path) {
                    d.logger.Info("✓ Traefik log source detected", d.logger.Args("path", path))
                    sources = append(sources, &models.LogSource{
                        Name:       generateName(path),
                        Path:       path,
                        ParserType: "traefik",
                    })
					// Only use the first valid source found
					break
                }else{
                    d.logger.WithCaller().Warn("Format invalid - not a Traefik access log", d.logger.Args("path", path))
                }
            } else {
				d.logger.Trace("File is directory or empty", d.logger.Args("path", path, "size", fileInfo.Size()))
			}
        } else {
			d.logger.Trace("File not accessible", d.logger.Args("path", path, "error", err.Error()))
		}
    }

	if len(sources) == 0 {
		d.logger.Warn("No Traefik log sources found. Check TRAEFIK_LOG_PATH in .env or ensure traefik/logs/access.log exists")
	}

    return sources, nil
}

func isTraefikFormat(path string) bool {
    file, err := os.Open(path)
    if err != nil {
        return false
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    if scanner.Scan() {
        line := scanner.Text()

        // Try JSON format first
        var logEntry map[string]any
        if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
            // Check for multiple Traefik-specific fields to improve detection accuracy
            // Traefik access logs typically contain these fields
            traefikFields := []string{"ClientHost", "RequestMethod", "RequestPath", "DownstreamStatus", "RouterName"}
            matchCount := 0

            for _, field := range traefikFields {
                if _, ok := logEntry[field]; ok {
                    matchCount++
                }
            }

            // If we find at least 2 Traefik-specific fields, consider it a Traefik log
            if matchCount >= 2 {
                return true
            }
        }

        // Try CLF format (both Traefik and generic)
        // Traefik CLF pattern: <client> - <userid> [<datetime>] "<method> <request> HTTP/<version>" <status> <size> "<referrer>" "<user_agent>" <requestsTotal> "<router>" "<server_URL>" <duration>ms
        traefikCLFPattern := `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+)? HTTP/[0-9.]+" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)" (\d+) "([^"]*)" "([^"]*)" (\d+)ms`
        if matched, _ := regexp.MatchString(traefikCLFPattern, line); matched {
            return true
        }

        // Generic CLF pattern: <client> - <userid> [<datetime>] "<method> <request> HTTP/<version>" <status> <size> "<referrer>" "<user_agent>"
        genericCLFPattern := `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+)? HTTP/[0-9.]+" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)"`
        if matched, _ := regexp.MatchString(genericCLFPattern, line); matched {
            return true
        }
    }
    return false
}

func generateName(path string) string {
    pathSplit := strings.Split(path, "/")
    fileNameExtension := pathSplit[(len(pathSplit)-1)]
    fileName := strings.Split(fileNameExtension, ".")[0]
    return "traefik-"+fileName
}