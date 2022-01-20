/*
 * Copyright (C) 2021 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package encode

import (
	"container/list"
	"encoding/json"
	"fmt"
	"github.com/netobserv/flowlogs2metrics/pkg/api"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const defaultExpiryTime = 120

type PromMetric struct {
	metricType  string
	promGauge   *prometheus.GaugeVec
	promCounter *prometheus.CounterVec
	promHist    *prometheus.HistogramVec
}

type metricInfo struct {
	input      string
	labelNames []string
	PromMetric
}

type entrySignature struct {
	Name   string
	Labels map[string]string
}

type entryInfo struct {
	eInfo entrySignature
	value float64
}

type metricCacheEntry struct {
	labels    prometheus.Labels
	timeStamp int64
	e         *list.Element
	key       string
	PromMetric
}

type metricCache map[string]*metricCacheEntry

type encodeProm struct {
	mu         sync.Mutex
	port       string
	prefix     string
	counters   map[string]metricInfo
	gauges     map[string]metricInfo
	histograms map[string]metricInfo
	expiryTime int64
	mList      *list.List
	mCache     metricCache
}

func (e *encodeProm) EncodeCounter(metric config.GenericMap) []interface{} {
	out := make([]interface{}, 0)
	for counterName, counterInfo := range e.counters {
		counterValue, ok := metric[counterInfo.input]
		if !ok {
			log.Debugf("field %v is missing", counterName)
			continue
		}
		counterValueString := fmt.Sprintf("%v", counterValue)
		valueFloat, err := strconv.ParseFloat(counterValueString, 64)
		if err != nil {
			log.Debugf("field cannot be converted to float: %v, %s", counterValue, counterValueString)
			continue
		}
		entryLabels := make(map[string]string, len(counterInfo.labelNames))
		for _, t := range counterInfo.labelNames {
			entryLabels[t] = fmt.Sprintf("%v", metric[t])
		}
		entry := entryInfo{
			eInfo: entrySignature{
				Name:   e.prefix + counterName,
				Labels: entryLabels,
			},
			value: valueFloat,
		}
		out = append(out, entry)
		// push the metric to prometheus
		if counterInfo.promCounter != nil {
			counterInfo.promCounter.With(entryLabels).Add(valueFloat)
		}
		cEntry := e.saveEntryInCache(entry, entryLabels)
		cEntry.PromMetric.metricType = api.PromEncodeOperationName("Counter")
		cEntry.PromMetric.promCounter = counterInfo.promCounter
	}
	return out
}
func (e *encodeProm) EncodeGauge(metric config.GenericMap) []interface{} {
	out := make([]interface{}, 0)
	for gaugeName, gaugeInfo := range e.gauges {
		gaugeValue, ok := metric[gaugeInfo.input]
		if !ok {
			log.Debugf("field %v is missing", gaugeName)
			continue
		}
		gaugeValueString := fmt.Sprintf("%v", gaugeValue)
		valueFloat, err := strconv.ParseFloat(gaugeValueString, 64)
		if err != nil {
			log.Debugf("field cannot be converted to float: %v, %s", gaugeValue, gaugeValueString)
			continue
		}
		entryLabels := make(map[string]string, len(gaugeInfo.labelNames))
		for _, t := range gaugeInfo.labelNames {
			entryLabels[t] = fmt.Sprintf("%v", metric[t])
		}
		entry := entryInfo{
			eInfo: entrySignature{
				Name:   e.prefix + gaugeName,
				Labels: entryLabels,
			},
			value: valueFloat,
		}
		out = append(out, entry)
		// push the metric to prometheus
		if gaugeInfo.promGauge != nil {
			gaugeInfo.promGauge.With(entryLabels).Set(valueFloat)
		}

		cEntry := e.saveEntryInCache(entry, entryLabels)
		cEntry.PromMetric.metricType = api.PromEncodeOperationName("Gauge")
		cEntry.PromMetric.promGauge = gaugeInfo.promGauge
	}
	return out
}

func generateCacheKey(sig *entrySignature) string {
	eInfoString := fmt.Sprintf("%s%v", sig.Name, sig.Labels)
	log.Debugf("generateCacheKey: eInfoString = %s", eInfoString)
	return eInfoString
}

func (e *encodeProm) saveEntryInCache(entry entryInfo, entryLabels map[string]string) *metricCacheEntry {
	// save item in cache; use eInfo as key to the cache
	var cEntry *metricCacheEntry
	nowInSecs := time.Now().Unix()
	eInfoString := generateCacheKey(&entry.eInfo)
	cEntry, ok := e.mCache[eInfoString]
	if ok {
		// item already exists in cache; update the element and move to end of list
		cEntry.timeStamp = nowInSecs
		// move to end of list
		e.mList.MoveToBack(cEntry.e)
	} else {
		// create new entry for cache
		cEntry = &metricCacheEntry{
			labels:    entryLabels,
			timeStamp: nowInSecs,
			key:       eInfoString,
		}
		// place at end of list
		log.Debugf("adding entry = %v", cEntry)
		cEntry.e = e.mList.PushBack(cEntry)
		e.mCache[eInfoString] = cEntry
		log.Debugf("mlist = %v", e.mList)
	}
	return cEntry
}

// Encode encodes a metric before being stored
func (e *encodeProm) Encode(metrics []config.GenericMap) []interface{} {
	log.Debugf("entering encodeProm Encode")
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]interface{}, 0)
	for _, metric := range metrics {
		gaugeOut := e.EncodeGauge(metric)
		out = append(out, gaugeOut...)
		counterOut := e.EncodeCounter(metric)
		out = append(out, counterOut...)
	}
	log.Debugf("cache = %v", e.mCache)
	log.Debugf("list = %v", e.mList)
	return out
}

func (e *encodeProm) cleanupExpiredEntriesLoop() {
	for {
		e.cleanupExpiredEntries()
		time.Sleep(time.Duration(e.expiryTime) * time.Second)
	}
}

// cleanupExpiredEntries - any entry that has expired should be removed from the prometheus reporting and cache
func (e *encodeProm) cleanupExpiredEntries() {
	log.Debugf("entering cleanupExpiredEntries")
	e.mu.Lock()
	defer e.mu.Unlock()
	log.Debugf("cache = %v", e.mCache)
	log.Debugf("list = %v", e.mList)
	nowInSecs := time.Now().Unix()
	expireTime := nowInSecs - e.expiryTime
	// go through the list until we reach recently used connections
	for {
		entry := e.mList.Front()
		if entry == nil {
			return
		}
		c := entry.Value.(*metricCacheEntry)
		log.Debugf("timeStamp = %d, expireTime = %d", c.timeStamp, expireTime)
		log.Debugf("c = %v", c)
		if c.timeStamp > expireTime {
			// no more expired items
			return
		}

		// clean up the entry
		log.Debugf("nowInSecs = %d, deleting %s", nowInSecs, c.labels)
		switch c.PromMetric.metricType {
		case api.PromEncodeOperationName("Gauge"):
			c.PromMetric.promGauge.Delete(c.labels)
		case api.PromEncodeOperationName("Counter"):
			c.PromMetric.promCounter.Delete(c.labels)
		case api.PromEncodeOperationName("Histogram"):
			c.PromMetric.promHist.Delete(c.labels)
		}
		delete(e.mCache, c.key)
		e.mList.Remove(entry)
	}
}

// startPrometheusInterface listens for prometheus resource usage requests
func startPrometheusInterface(w *encodeProm) {
	log.Debugf("entering startPrometheusInterface")
	log.Infof("startPrometheusInterface: port num = %s", w.port)

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.
	http.Handle("/metrics", promhttp.Handler())

	err := http.ListenAndServe(w.port, nil)
	if err != nil {
		log.Errorf("error in http.ListenAndServe: %v", err)
		os.Exit(1)
	}
}

func NewEncodeProm() (Encoder, error) {
	encodePromString := config.Opt.PipeLine.Encode.Prom
	log.Debugf("promEncodeString = %s", encodePromString)
	var jsonEncodeProm api.PromEncode
	err := json.Unmarshal([]byte(encodePromString), &jsonEncodeProm)
	if err != nil {
		return nil, err
	}

	portNum := jsonEncodeProm.Port
	promPrefix := jsonEncodeProm.Prefix
	expiryTime := int64(jsonEncodeProm.ExpiryTime)
	if expiryTime == 0 {
		expiryTime = defaultExpiryTime
	}
	log.Debugf("expiryTime = %d", expiryTime)

	counters := make(map[string]metricInfo)
	gauges := make(map[string]metricInfo)
	histograms := make(map[string]metricInfo)
	for _, mInfo := range jsonEncodeProm.Metrics {
		fullMetricName := promPrefix + mInfo.Name
		labels := mInfo.Labels
		log.Debugf("fullMetricName = %v", fullMetricName)
		log.Debugf("Labels = %v", labels)
		switch mInfo.Type {
		case api.PromEncodeOperationName("Counter"):
			counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: fullMetricName, Help: ""}, labels)
			err := prometheus.Register(counter)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			counters[mInfo.Name] = metricInfo{
				input:      mInfo.ValueKey,
				labelNames: labels,
				PromMetric: PromMetric{
					metricType:  api.PromEncodeOperationName("Counter"),
					promCounter: counter,
				},
			}
		case api.PromEncodeOperationName("Gauge"):
			gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: fullMetricName, Help: ""}, labels)
			err := prometheus.Register(gauge)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			gauges[mInfo.Name] = metricInfo{
				input:      mInfo.ValueKey,
				labelNames: labels,
				PromMetric: PromMetric{
					metricType: api.PromEncodeOperationName("Gauge"),
					promGauge:  gauge,
				},
			}
		case api.PromEncodeOperationName("Histogram"):
			log.Debugf("buckets = %v", mInfo.Buckets)
			hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: fullMetricName, Help: "", Buckets: mInfo.Buckets}, labels)
			err := prometheus.Register(hist)
			if err != nil {
				log.Errorf("error during prometheus.Register: %v", err)
				return nil, err
			}
			histograms[mInfo.Name] = metricInfo{
				input:      mInfo.ValueKey,
				labelNames: labels,
				PromMetric: PromMetric{
					metricType: api.PromEncodeOperationName("Histogram"),
					promHist:   hist,
				},
			}
		case "default":
			log.Errorf("invalid metric type = %v, skipping", mInfo.Type)
		}
	}

	w := &encodeProm{
		port:       fmt.Sprintf(":%v", portNum),
		prefix:     promPrefix,
		counters:   counters,
		gauges:     gauges,
		histograms: histograms,
		expiryTime: expiryTime,
		mList:      list.New(),
		mCache:     make(metricCache),
	}
	go startPrometheusInterface(w)
	go w.cleanupExpiredEntriesLoop()
	return w, nil
}
