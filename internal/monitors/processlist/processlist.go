package processlist

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/signalfx/golib/event"
	"github.com/signalfx/signalfx-agent/internal/core/config"
	"github.com/signalfx/signalfx-agent/internal/monitors"
	"github.com/signalfx/signalfx-agent/internal/monitors/types"
	"github.com/signalfx/signalfx-agent/internal/utils"
	log "github.com/sirupsen/logrus"
)

const version = "0.0.30"

// EVENT(objects.top-info): Process list event.

var logger = log.WithFields(log.Fields{"monitorType": monitorType})
var zlibCompressor = zlib.NewWriter(&bytes.Buffer{})

// Config for this monitor
type Config struct {
	config.MonitorConfig `singleInstance:"true" acceptsEndpoints:"false"`
}

func init() {
	monitors.Register(&monitorMetadata, func() interface{} { return &Monitor{} }, &Config{})
}

// compresses the given byte array
func compressBytes(in []byte) (buf bytes.Buffer, err error) {
	zlibCompressor.Reset(&buf)
	_, err = zlibCompressor.Write(in)
	_ = zlibCompressor.Close()
	return
}

// Monitor for Utilization
type Monitor struct {
	Output types.Output
	cancel func()
}

// TopProcess is a platform-independent way of representing a process to be
// reported to SignalFx
type TopProcess struct {
	ProcessID           int
	Username            string
	Priority            int
	Nice                string
	PageFileUsageBytes  int
	WorkingSetSizeBytes int
	SharedMemBytes      int
	Status              string
	CPUPercent          float64
	MemPercent          float64
	TotalCPUTime        time.Duration
	Command             string
}

// Configure configures the monitor and starts collecting on the configured interval
func (m *Monitor) Configure(conf *Config) error {
	// create contexts for managing the the plugin loop
	var ctx context.Context
	ctx, m.cancel = context.WithCancel(context.Background())

	utils.RunOnInterval(
		ctx,
		func() {
			// get the process list
			procs, err := ProcessList()
			if err != nil {
				logger.WithError(err).Error("Couldn't get process list")
				return
			}

			message, err := encodeEventMessage(procs)
			if err != nil {
				logger.WithError(err).Error("Failed to encode process list")
			}

			m.Output.SendEvent(
				&event.Event{
					EventType:  "objects.top-info",
					Category:   event.AGENT,
					Dimensions: map[string]string{},
					Properties: map[string]interface{}{
						"message": message,
					},
					Timestamp: time.Now(),
				},
			)
		},
		time.Duration(conf.IntervalSeconds)*time.Second,
	)
	return nil
}

func encodeEventMessage(procs []*TopProcess) (string, error) {
	if len(procs) == 0 {
		return "", errors.New("no processes to encode")
	}

	procsEncoded := []byte{'{'}
	for i := range procs {
		procsEncoded = append(procsEncoded, []byte(encodeProcess(procs[i])+",")...)
	}
	procsEncoded[len(procsEncoded)-1] = '}'

	// escape and compress the process list
	escapedBytes := bytes.Replace(procsEncoded, []byte{byte('\\')}, []byte{byte('\\'), byte('\\')}, -1)
	compressedBytes, err := compressBytes(escapedBytes)
	if err != nil {
		return "", fmt.Errorf("couldn't compress process list: %v", err)
	}

	return fmt.Sprintf(
		"{\"t\":\"%s\",\"v\":\"%s\"}",
		base64.StdEncoding.EncodeToString(compressedBytes.Bytes()), version), nil
}

func encodeProcess(proc *TopProcess) string {
	return fmt.Sprintf("\"%d\":[\"%s\",%d,\"%s\",%d,%d,%d,\"%s\",%.2f,%.2f,\"%s\",\"%s\"]",
		proc.ProcessID,
		proc.Username,
		proc.Priority,
		proc.Nice,
		proc.PageFileUsageBytes/1024,
		proc.WorkingSetSizeBytes/1024,
		proc.SharedMemBytes,
		proc.Status,
		proc.CPUPercent,
		proc.MemPercent,
		toTime(proc.TotalCPUTime.Seconds()),
		proc.Command,
	)
}

// toTime returns the given seconds as a formatted string "min:sec.dec"
func toTime(secs float64) string {
	minutes := int(secs) / 60
	seconds := math.Mod(secs, 60.0)
	dec := math.Mod(seconds, 1.0) * 100
	return fmt.Sprintf("%02d:%02.f.%02.f", minutes, seconds, dec)
}
