// MIT License
//
// Copyright (c) 2026 Kolin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package haproxy

import "time"

// AccessLogEvent represents a parsed HAProxy HTTP access log entry.
// Field names match HTTPRequest model fields for reflection-based mapping.
type AccessLogEvent struct {
	Timestamp  time.Time
	SourceName string

	ClientIP   string
	ClientPort int

	Method      string
	Protocol    string
	Path        string
	QueryString string

	StatusCode     int
	ResponseSize   int64
	ResponseTimeMs float64
	Duration       int64
	StartUTC       string

	BackendName string
	BackendURL  string
	RouterName  string

	RetryAttempts int
}

func (e *AccessLogEvent) GetTimestamp() time.Time { return e.Timestamp }
func (e *AccessLogEvent) GetSourceName() string    { return e.SourceName }
