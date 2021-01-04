// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlphttp_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/internal/otlptest"
	"go.opentelemetry.io/otel/exporters/otlp/otlphttp"
)

const (
	relOtherMetricsPath = "post/metrics/here"
	relOtherTracesPath  = "post/traces/here"
	otherMetricsPath    = "/post/metrics/here"
	otherTracesPath     = "/post/traces/here"
)

func TestEndToEnd(t *testing.T) {
	tests := []struct {
		name  string
		opts  []otlphttp.Option
		mcCfg mockCollectorConfig
	}{
		{
			name: "no extra options",
			opts: nil,
		},
		{
			name: "with gzip compression",
			opts: []otlphttp.Option{
				otlphttp.WithCompression(otlphttp.GzipCompression),
			},
		},
		{
			name: "with different paths",
			opts: []otlphttp.Option{
				otlphttp.WithMetricsURLPath(otherMetricsPath),
				otlphttp.WithTracesURLPath(otherTracesPath),
			},
			mcCfg: mockCollectorConfig{
				MetricsURLPath: otherMetricsPath,
				TracesURLPath:  otherTracesPath,
			},
		},
		{
			name: "with relative paths",
			opts: []otlphttp.Option{
				otlphttp.WithMetricsURLPath(relOtherMetricsPath),
				otlphttp.WithTracesURLPath(relOtherTracesPath),
			},
			mcCfg: mockCollectorConfig{
				MetricsURLPath: otherMetricsPath,
				TracesURLPath:  otherTracesPath,
			},
		},
	}

	for _, tc := range tests {
		func() {
			mc := runMockCollector(t, tc.mcCfg)
			defer mc.MustStop(t)
			allOpts := append([]otlphttp.Option{
				otlphttp.WithEndpoint(mc.Endpoint()),
				otlphttp.WithInsecure(),
			}, tc.opts...)
			driver := otlphttp.NewDriver(allOpts...)
			ctx := context.Background()
			exporter, err := otlp.NewExporter(ctx, driver)
			if assert.NoError(t, err) {
				defer func() {
					assert.NoError(t, exporter.Shutdown(ctx))
				}()
				otlptest.RunEndToEndTest(ctx, t, exporter, mc, mc)
			}
		}()
	}
}

func TestRetry(t *testing.T) {
	statuses := []int{
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
	}
	mcCfg := mockCollectorConfig{
		InjectHTTPStatus: statuses,
	}
	mc := runMockCollector(t, mcCfg)
	defer mc.MustStop(t)
	driver := otlphttp.NewDriver(
		otlphttp.WithEndpoint(mc.Endpoint()),
		otlphttp.WithInsecure(),
		otlphttp.WithMaxAttempts(len(statuses)+1),
	)
	ctx := context.Background()
	exporter, err := otlp.NewExporter(ctx, driver)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, exporter.Shutdown(ctx))
	}()
	err = exporter.ExportSpans(ctx, otlptest.SingleSpanSnapshot())
	assert.NoError(t, err)
	assert.Len(t, mc.GetSpans(), 1)
}

func TestRetryFailed(t *testing.T) {
	statuses := []int{
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
	}
	mcCfg := mockCollectorConfig{
		InjectHTTPStatus: statuses,
	}
	mc := runMockCollector(t, mcCfg)
	defer mc.MustStop(t)
	driver := otlphttp.NewDriver(
		otlphttp.WithEndpoint(mc.Endpoint()),
		otlphttp.WithInsecure(),
		otlphttp.WithMaxAttempts(1),
	)
	ctx := context.Background()
	exporter, err := otlp.NewExporter(ctx, driver)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, exporter.Shutdown(ctx))
	}()
	err = exporter.ExportSpans(ctx, otlptest.SingleSpanSnapshot())
	assert.Error(t, err)
	assert.Empty(t, mc.GetSpans())
}

func TestNoRetry(t *testing.T) {
	statuses := []int{
		http.StatusBadRequest,
	}
	mcCfg := mockCollectorConfig{
		InjectHTTPStatus: statuses,
	}
	mc := runMockCollector(t, mcCfg)
	defer mc.MustStop(t)
	driver := otlphttp.NewDriver(
		otlphttp.WithEndpoint(mc.Endpoint()),
		otlphttp.WithInsecure(),
		otlphttp.WithMaxAttempts(len(statuses)+1),
	)
	ctx := context.Background()
	exporter, err := otlp.NewExporter(ctx, driver)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, exporter.Shutdown(ctx))
	}()
	err = exporter.ExportSpans(ctx, otlptest.SingleSpanSnapshot())
	assert.Error(t, err)
	assert.Empty(t, mc.GetSpans())
}

func TestFailedCheckpoint(t *testing.T) {
	mcCfg := mockCollectorConfig{}
	mc := runMockCollector(t, mcCfg)
	defer mc.MustStop(t)
	driver := otlphttp.NewDriver(
		otlphttp.WithEndpoint(mc.Endpoint()),
		otlphttp.WithInsecure(),
	)
	ctx := context.Background()
	exporter, err := otlp.NewExporter(ctx, driver)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, exporter.Shutdown(ctx))
	}()
	err = exporter.Export(ctx, otlptest.FailCheckpointSet{})
	assert.Error(t, err)
	assert.Empty(t, mc.GetMetrics())
}

func TestEmptyData(t *testing.T) {
	mcCfg := mockCollectorConfig{}
	mc := runMockCollector(t, mcCfg)
	defer mc.MustStop(t)
	driver := otlphttp.NewDriver(
		otlphttp.WithEndpoint(mc.Endpoint()),
		otlphttp.WithInsecure(),
	)
	ctx := context.Background()
	exporter, err := otlp.NewExporter(ctx, driver)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, exporter.Shutdown(ctx))
	}()
	err = exporter.Export(ctx, otlptest.EmptyCheckpointSet{})
	assert.NoError(t, err)
	err = exporter.ExportSpans(ctx, nil)
	assert.NoError(t, err)
	assert.Empty(t, mc.GetMetrics())
	assert.Empty(t, mc.GetSpans())
}

func TestUnreasonableMaxAttempts(t *testing.T) {
	// Max attempts is 5, we set collector to fail 7 times and try
	// to configure max attempts to be 10. Since we cap max
	// attempts to 5, exporting to the collector should fail.
	statuses := make([]int, 0, 7)
	for i := 0; i < cap(statuses); i++ {
		statuses = append(statuses, http.StatusTooManyRequests)
	}
	mcCfg := mockCollectorConfig{
		InjectHTTPStatus: statuses,
	}
	mc := runMockCollector(t, mcCfg)
	defer mc.MustStop(t)
	driver := otlphttp.NewDriver(
		otlphttp.WithEndpoint(mc.Endpoint()),
		otlphttp.WithInsecure(),
		otlphttp.WithMaxAttempts(10),
		otlphttp.WithBackoff(time.Millisecond),
	)
	ctx := context.Background()
	exporter, err := otlp.NewExporter(ctx, driver)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, exporter.Shutdown(ctx))
	}()
	err = exporter.ExportSpans(ctx, otlptest.SingleSpanSnapshot())
	assert.Error(t, err)
	assert.Empty(t, mc.GetSpans())
}

func TestCancelledContext(t *testing.T) {
	mcCfg := mockCollectorConfig{}
	mc := runMockCollector(t, mcCfg)
	defer mc.MustStop(t)
	driver := otlphttp.NewDriver(
		otlphttp.WithEndpoint(mc.Endpoint()),
		otlphttp.WithInsecure(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	exporter, err := otlp.NewExporter(ctx, driver)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, exporter.Shutdown(ctx))
	}()
	cancel()
	err = exporter.ExportSpans(ctx, otlptest.SingleSpanSnapshot())
	assert.Error(t, err)
	assert.Empty(t, mc.GetSpans())
}

func TestDeadlineContext(t *testing.T) {
	statuses := make([]int, 0, 5)
	for i := 0; i < cap(statuses); i++ {
		statuses = append(statuses, http.StatusTooManyRequests)
	}
	mcCfg := mockCollectorConfig{
		InjectHTTPStatus: statuses,
	}
	mc := runMockCollector(t, mcCfg)
	defer mc.MustStop(t)
	driver := otlphttp.NewDriver(
		otlphttp.WithEndpoint(mc.Endpoint()),
		otlphttp.WithInsecure(),
		otlphttp.WithBackoff(time.Minute),
	)
	ctx := context.Background()
	exporter, err := otlp.NewExporter(ctx, driver)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, exporter.Shutdown(ctx))
	}()
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	err = exporter.ExportSpans(ctx, otlptest.SingleSpanSnapshot())
	assert.Error(t, err)
	assert.Empty(t, mc.GetSpans())
}

func TestStopWhileExporting(t *testing.T) {
	statuses := make([]int, 0, 5)
	for i := 0; i < cap(statuses); i++ {
		statuses = append(statuses, http.StatusTooManyRequests)
	}
	mcCfg := mockCollectorConfig{
		InjectHTTPStatus: statuses,
	}
	mc := runMockCollector(t, mcCfg)
	defer mc.MustStop(t)
	driver := otlphttp.NewDriver(
		otlphttp.WithEndpoint(mc.Endpoint()),
		otlphttp.WithInsecure(),
		otlphttp.WithBackoff(time.Minute),
	)
	ctx := context.Background()
	exporter, err := otlp.NewExporter(ctx, driver)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, exporter.Shutdown(ctx))
	}()
	doneCh := make(chan struct{})
	go func() {
		err := exporter.ExportSpans(ctx, otlptest.SingleSpanSnapshot())
		assert.Error(t, err)
		assert.Empty(t, mc.GetSpans())
		close(doneCh)
	}()
	<-time.After(time.Second)
	err = exporter.Shutdown(ctx)
	assert.NoError(t, err)
	<-doneCh
}