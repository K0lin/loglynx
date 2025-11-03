package useragent

import (
	"regexp"
	"strings"
)

// UserAgentInfo contains parsed information from a User-Agent string
type UserAgentInfo struct {
	Browser        string
	BrowserVersion string
	OS             string
	OSVersion      string
	DeviceType     string
}

var (
	// Browser patterns (order matters - more specific first)
	browserPatterns = []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"Edge", regexp.MustCompile(`(?i)Edg/(\d+\.\d+)`)},
		{"Chrome", regexp.MustCompile(`(?i)Chrome/(\d+\.\d+)`)},
		{"Safari", regexp.MustCompile(`(?i)Version/(\d+\.\d+).*Safari`)},
		{"Firefox", regexp.MustCompile(`(?i)Firefox/(\d+\.\d+)`)},
		{"Opera", regexp.MustCompile(`(?i)(?:Opera|OPR)/(\d+\.\d+)`)},
		{"IE", regexp.MustCompile(`(?i)MSIE\s+(\d+\.\d+)`)},
		{"IE", regexp.MustCompile(`(?i)Trident/.*rv:(\d+\.\d+)`)},
	}

	// OS patterns
	osPatterns = []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"Windows", regexp.MustCompile(`(?i)Windows NT (\d+\.\d+)`)},
		{"macOS", regexp.MustCompile(`(?i)Mac OS X (\d+[._]\d+)`)},
		{"iOS", regexp.MustCompile(`(?i)(?:iPhone|iPad).*OS (\d+[._]\d+)`)},
		{"Android", regexp.MustCompile(`(?i)Android (\d+\.\d+)`)},
		{"Linux", regexp.MustCompile(`(?i)Linux`)},
		{"ChromeOS", regexp.MustCompile(`(?i)CrOS`)},
	}

	// Bot patterns
	botPatterns = regexp.MustCompile(`(?i)bot|crawler|spider|scraper|curl|wget|python|go-http`)

	// Mobile patterns
	mobilePatterns = regexp.MustCompile(`(?i)mobile|android|iphone|ipad|ipod|blackberry|windows phone`)
)

// Parse extracts browser, OS, and device information from a User-Agent string
func Parse(userAgent string) *UserAgentInfo {
	if userAgent == "" {
		return &UserAgentInfo{
			Browser:    "Unknown",
			OS:         "Unknown",
			DeviceType: "unknown",
		}
	}

	info := &UserAgentInfo{}

	// Check if it's a bot
	if botPatterns.MatchString(userAgent) {
		info.DeviceType = "bot"
		info.Browser = detectBot(userAgent)
		info.OS = "Bot"
		return info
	}

	// Detect browser
	for _, bp := range browserPatterns {
		if matches := bp.pattern.FindStringSubmatch(userAgent); matches != nil {
			info.Browser = bp.name
			if len(matches) > 1 {
				info.BrowserVersion = matches[1]
			}
			break
		}
	}
	if info.Browser == "" {
		info.Browser = "Unknown"
	}

	// Detect OS
	for _, op := range osPatterns {
		if matches := op.pattern.FindStringSubmatch(userAgent); matches != nil {
			info.OS = op.name
			if len(matches) > 1 {
				// Replace underscores with dots for iOS versions
				info.OSVersion = strings.ReplaceAll(matches[1], "_", ".")
			}
			break
		}
	}
	if info.OS == "" {
		info.OS = "Unknown"
	}

	// Detect device type
	if mobilePatterns.MatchString(userAgent) {
		info.DeviceType = "mobile"
	} else {
		info.DeviceType = "desktop"
	}

	return info
}

// detectBot tries to identify the specific bot from User-Agent
func detectBot(userAgent string) string {
	botNames := map[string]string{
		"googlebot":    "Googlebot",
		"bingbot":      "Bingbot",
		"slurp":        "Yahoo Slurp",
		"duckduckbot":  "DuckDuckBot",
		"baiduspider":  "Baidu Spider",
		"yandexbot":    "YandexBot",
		"facebookbot":  "Facebookbot",
		"twitterbot":   "Twitterbot",
		"linkedinbot":  "LinkedInBot",
		"applebot":     "Applebot",
		"python":       "Python Client",
		"go-http":      "Go HTTP Client",
		"curl":         "cURL",
		"wget":         "Wget",
		"postman":      "Postman",
	}

	lowerUA := strings.ToLower(userAgent)
	for pattern, name := range botNames {
		if strings.Contains(lowerUA, pattern) {
			return name
		}
	}

	return "Bot"
}

// GetWindowsVersion converts Windows NT version to friendly name
func GetWindowsVersion(ntVersion string) string {
	versions := map[string]string{
		"10.0": "Windows 10/11",
		"6.3":  "Windows 8.1",
		"6.2":  "Windows 8",
		"6.1":  "Windows 7",
		"6.0":  "Windows Vista",
		"5.1":  "Windows XP",
	}

	if friendly, ok := versions[ntVersion]; ok {
		return friendly
	}
	return "Windows " + ntVersion
}
