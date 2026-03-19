package log_analyzer

import (
	"bufio"
	"fmt"
	"math"
	"mime/multipart"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity of a log entry
type LogLevel string

const (
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
	FATAL LogLevel = "FATAL"
	DEBUG LogLevel = "DEBUG"
)

// LogEntry is a structured representation of a single log line
type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Source    string
	Service   string
	Message   string
	Raw       string
	LineNum   int
}

// AnalysisResult holds the summary and details of the log analysis
type AnalysisResult struct {
	TotalLines      int
	ErrorCount      int
	WarningCount    int
	StartTime       time.Time
	EndTime         time.Time
	Entries         []LogEntry // Only keep relevant/recent entries to save RAM
	ErrorGroups     map[string]int
	RootCauses      []RootCause
	Timeline        []LogEntry // Merged timeline
	Clusters        []LogCluster
	Anomalies       []Anomaly
}

// LogCluster represents a group of similar log messages
type LogCluster struct {
	Pattern    string
	Sample     string
	Count      int
	Percentage float64
	FirstSeen  time.Time
	LastSeen   time.Time
}

// Anomaly represents a statistical anomaly in log volume
type Anomaly struct {
	Timestamp time.Time
	LogCount  int
	Expected  float64
	Deviation float64 // How many standard deviations away
	Severity  string  // Low, Medium, High
}

// RootCause suggests a potential reason for an issue
type RootCause struct {
	Title       string
	Description string
	Confidence  string // High, Medium, Low
	Count       int
}

// AnalyzerService handles the logic for parsing and analyzing logs
type AnalyzerService struct {
	// Max lines to keep in memory for timeline
	MaxTimelineLines int
}

func NewAnalyzerService() *AnalyzerService {
	return &AnalyzerService{
		MaxTimelineLines: 50000,
	}
}

// ParseAndAnalyze processes multiple log files
func (s *AnalyzerService) ParseAndAnalyze(files []*multipart.FileHeader) (*AnalysisResult, error) {
	var wg sync.WaitGroup
	resultsChan := make(chan []LogEntry, len(files))
	errorsChan := make(chan error, len(files))

	// Parallel Processing
	for _, fileHeader := range files {
		wg.Add(1)
		go func(fh *multipart.FileHeader) {
			defer wg.Done()
			entries, err := s.parseFile(fh)
			if err != nil {
				errorsChan <- err
				return
			}
			resultsChan <- entries
		}(fileHeader)
	}

	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	// Check for errors
	if len(errorsChan) > 0 {
		return nil, <-errorsChan // Return first error for simplicity
	}

	// Merge Results
	var allEntries []LogEntry
	for entries := range resultsChan {
		allEntries = append(allEntries, entries...)
	}

	// Sort by Timestamp
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.Before(allEntries[j].Timestamp)
	})

	// Analyze
	result := s.analyzeEntries(allEntries)
	return result, nil
}

func (s *AnalyzerService) parseFile(fh *multipart.FileHeader) ([]LogEntry, error) {
	file, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(file)
	// Buffer for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0
	
	// Regex for common log formats
	// Example: 2024-01-01 10:00:00 [INFO] Service: Message
	// Example: [2024-01-01T10:00:00Z] INFO: Message
	reTimestamp := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)`)
	reLevel := regexp.MustCompile(`(INFO|WARN|ERROR|FATAL|DEBUG)`)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		entry := LogEntry{
			Raw:     line,
			LineNum: lineNum,
			Source:  fh.Filename,
			Level:   INFO, // Default
		}

		// Extract Timestamp
		tsMatch := reTimestamp.FindString(line)
		if tsMatch != "" {
			// Try parsing multiple formats
			formats := []string{
				"2006-01-02 15:04:05",
				time.RFC3339,
				"2006-01-02T15:04:05",
			}
			for _, layout := range formats {
				t, err := time.Parse(layout, tsMatch)
				if err == nil {
					entry.Timestamp = t
					break
				}
			}
		}

		// Extract Level
		lvlMatch := reLevel.FindString(line)
		if lvlMatch != "" {
			entry.Level = LogLevel(lvlMatch)
		}

		// Extract Message (Simple split for now)
		// Assuming format often puts message at end
		// For now, raw line is message if no structure detected
		entry.Message = line

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func (s *AnalyzerService) analyzeEntries(entries []LogEntry) *AnalysisResult {
	result := &AnalysisResult{
		TotalLines:  len(entries),
		ErrorGroups: make(map[string]int),
		RootCauses:  make([]RootCause, 0),
	}

	if len(entries) == 0 {
		return result
	}

	result.StartTime = entries[0].Timestamp
	result.EndTime = entries[len(entries)-1].Timestamp

	// Limit stored entries for memory efficiency
	if len(entries) > s.MaxTimelineLines {
		result.Timeline = entries[len(entries)-s.MaxTimelineLines:]
	} else {
		result.Timeline = entries
	}
	result.Entries = result.Timeline

	// Pattern Recognition Counters
	var (
		consecutiveTimeouts int
		consecutiveRestarts int
		dbErrors            int
		memoryErrors        int
		networkErrors       int
	)

	for _, entry := range entries {
		msg := strings.ToLower(entry.Message)

		// Count Levels
		if entry.Level == ERROR || entry.Level == FATAL {
			result.ErrorCount++
			// Group errors (simplified)
			shortMsg := s.simplifyError(msg)
			result.ErrorGroups[shortMsg]++
		} else if entry.Level == WARN {
			result.WarningCount++
		}

		// Detect specific patterns
		if strings.Contains(msg, "timeout") {
			consecutiveTimeouts++
			networkErrors++
		} else {
			consecutiveTimeouts = 0
		}

		if strings.Contains(msg, "restart") || strings.Contains(msg, "starting service") {
			consecutiveRestarts++
		}

		if strings.Contains(msg, "database") || strings.Contains(msg, "sql") || strings.Contains(msg, "connection refused") {
			dbErrors++
		}

		if strings.Contains(msg, "out of memory") || strings.Contains(msg, "oom") || strings.Contains(msg, "heap") {
			memoryErrors++
		}
	}

	// Generate Root Causes
	if consecutiveTimeouts > 3 {
		result.RootCauses = append(result.RootCauses, RootCause{
			Title:       "Network Instability / High Latency",
			Description: fmt.Sprintf("Detected %d consecutive timeouts. Possible network congestion or downstream service failure.", consecutiveTimeouts),
			Confidence:  "High",
			Count:       consecutiveTimeouts,
		})
	}

	if dbErrors > 0 {
		result.RootCauses = append(result.RootCauses, RootCause{
			Title:       "Database Connection Issues",
			Description: "Multiple database-related errors found. Check DB status and connection string.",
			Confidence:  "Medium",
			Count:       dbErrors,
		})
	}

	if memoryErrors > 0 {
		result.RootCauses = append(result.RootCauses, RootCause{
			Title:       "Memory Resource Exhaustion",
			Description: "OOM or Heap errors detected. Application may be leaking memory or under-provisioned.",
			Confidence:  "High",
			Count:       memoryErrors,
		})
	}
    
	// Fallback if errors exist but no specific pattern
	if result.ErrorCount > 0 && len(result.RootCauses) == 0 {
		// Find top error
		var topError string
		var maxCount int
		for k, v := range result.ErrorGroups {
			if v > maxCount {
				maxCount = v
				topError = k
			}
		}
		if topError != "" {
			result.RootCauses = append(result.RootCauses, RootCause{
				Title:       "Recurring Application Error",
				Description: fmt.Sprintf("Most frequent error: %s", topError),
				Confidence:  "Medium",
				Count:       maxCount,
			})
		}
	}

	// Perform Advanced ML Analysis
	result.Clusters = s.clusterLogs(entries)
	result.Anomalies = s.detectAnomalies(entries)

	return result
}

func (s *AnalyzerService) simplifyError(msg string) string {
	// Simple heuristic to group similar errors
	// Take first 50 chars or split by common delimiters
	if len(msg) > 60 {
		return msg[:60] + "..."
	}
	return msg
}

// AnswerUserQuestion searches logs for keywords related to user query
func (s *AnalyzerService) AnswerUserQuestion(entries []LogEntry, question string) []RootCause {
	question = strings.ToLower(question)
	keywords := strings.Fields(question)
	
	// Remove common stop words
	stopWords := map[string]bool{"why": true, "is": true, "the": true, "failed": true, "crash": true}
	var searchTerms []string
	for _, k := range keywords {
		if !stopWords[k] {
			searchTerms = append(searchTerms, k)
		}
	}
    
    if len(searchTerms) == 0 {
        // If question was generic like "why crash", assume we look for fatal/error
        searchTerms = []string{"fatal", "panic", "error", "exception"}
    }

	var matches int
	var relevantEntries []LogEntry

	for _, entry := range entries {
		msg := strings.ToLower(entry.Message)
		for _, term := range searchTerms {
			if strings.Contains(msg, term) {
				matches++
				relevantEntries = append(relevantEntries, entry)
				break
			}
		}
	}
    
    // Sort relevant entries by timestamp descending to find most recent cause
    // (Already sorted, so just take last ones)
    
    var causes []RootCause
    if matches > 0 {
        // Analyze the relevant entries
        lastEntry := relevantEntries[len(relevantEntries)-1]
        causes = append(causes, RootCause{
            Title: "Found relevant log entries",
            Description: fmt.Sprintf("Found %d matches. Last occurrence: [%s] %s", matches, lastEntry.Timestamp.Format("15:04:05"), s.simplifyError(lastEntry.Message)),
            Confidence: "Medium",
            Count: matches,
        })
    } else {
         causes = append(causes, RootCause{
            Title: "No direct matches found",
            Description: "Try using different keywords or checking the error summary.",
            Confidence: "Low",
            Count: 0,
        })
    }

	return causes
}

// clusterLogs groups similar logs by masking variables
func (s *AnalyzerService) clusterLogs(entries []LogEntry) []LogCluster {
	clusters := make(map[string]*LogCluster)
	total := len(entries)
	if total == 0 {
		return nil
	}

	// Regex to mask variables: numbers, hex, timestamps, quoted strings, UUIDs
	reVar := regexp.MustCompile(`\d+|0x[0-9a-fA-F]+|'.*?'|".*?"|\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)

	for _, entry := range entries {
		// Create pattern by replacing variables with placeholders
		// Limit message length for performance
		msg := entry.Message
		if len(msg) > 500 {
			msg = msg[:500]
		}
		pattern := reVar.ReplaceAllString(msg, "*")

		if _, exists := clusters[pattern]; !exists {
			clusters[pattern] = &LogCluster{
				Pattern:   pattern,
				Sample:    entry.Message,
				FirstSeen: entry.Timestamp,
				LastSeen:  entry.Timestamp,
			}
		}
		c := clusters[pattern]
		c.Count++
		if entry.Timestamp.Before(c.FirstSeen) {
			c.FirstSeen = entry.Timestamp
		}
		if entry.Timestamp.After(c.LastSeen) {
			c.LastSeen = entry.Timestamp
		}
	}

	var result []LogCluster
	for _, c := range clusters {
		c.Percentage = float64(c.Count) / float64(total) * 100
		result = append(result, *c)
	}

	// Sort by count desc
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	// Return top 20 clusters
	if len(result) > 20 {
		return result[:20]
	}
	return result
}

// detectAnomalies identifies time windows with unusual log volume
func (s *AnalyzerService) detectAnomalies(entries []LogEntry) []Anomaly {
	if len(entries) == 0 {
		return nil
	}

	// 1. Bucketize logs by minute
	buckets := make(map[int64]int)
	minTime := entries[0].Timestamp.Truncate(time.Minute).Unix()
	maxTime := entries[len(entries)-1].Timestamp.Truncate(time.Minute).Unix()

	for _, e := range entries {
		t := e.Timestamp.Truncate(time.Minute).Unix()
		buckets[t]++
		if t < minTime {
			minTime = t
		}
		if t > maxTime {
			maxTime = t
		}
	}

	// Fill gaps with 0 and create ordered slice
	var counts []float64
	var times []int64
	for t := minTime; t <= maxTime; t += 60 {
		count := float64(buckets[t])
		counts = append(counts, count)
		times = append(times, t)
	}

	if len(counts) < 5 {
		return nil // Not enough data points for statistical analysis
	}

	// 2. Calculate Stats (Mean, StdDev)
	var sum float64
	for _, c := range counts {
		sum += c
	}
	n := float64(len(counts))
	mean := sum / n

	var sumSqDiff float64
	for _, c := range counts {
		diff := c - mean
		sumSqDiff += diff * diff
	}
	variance := sumSqDiff / n
	stdDev := math.Sqrt(variance)

	// Avoid division by zero if logs are perfectly constant
	if stdDev == 0 {
		return nil
	}

	// 3. Detect Anomalies (Rule: > Mean + 2*StdDev)
	var anomalies []Anomaly
	threshold := mean + (2 * stdDev)

	for i, count := range counts {
		if count > threshold {
			deviation := (count - mean) / stdDev
			severity := "Low"
			if deviation > 4 {
				severity = "High"
			} else if deviation > 3 {
				severity = "Medium"
			}

			anomalies = append(anomalies, Anomaly{
				Timestamp: time.Unix(times[i], 0),
				LogCount:  int(count),
				Expected:  mean,
				Deviation: deviation,
				Severity:  severity,
			})
		}
	}

	// Sort by deviation desc
	sort.Slice(anomalies, func(i, j int) bool {
		return anomalies[i].Deviation > anomalies[j].Deviation
	})

	return anomalies
}
