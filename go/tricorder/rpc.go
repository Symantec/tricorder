package tricorder

import (
	"github.com/Symantec/tricorder/go/tricorder/messages"
	"net/rpc"
)

func rpcAsMetric(m *metric, s *session) *messages.Metric {
	var result messages.Metric
	m.InitRpcMetric(s, &result)
	return &result
}

type rpcMetricsCollector messages.MetricList

func (c *rpcMetricsCollector) Collect(m *metric, s *session) (err error) {
	*c = append(*c, rpcAsMetric(m, s))
	return nil
}

type rpcType int

func (t *rpcType) ListMetrics(path string, response *messages.MetricList) error {
	return root.GetAllMetricsByPath(
		path, (*rpcMetricsCollector)(response), nil)
}

func (t *rpcType) GetMetric(path string, response *messages.Metric) error {
	m := root.GetMetric(path)
	if m == nil {
		return messages.ErrMetricNotFound
	}
	*response = *rpcAsMetric(m, nil)
	return nil
}

func initRpcHandlers() {
	rpc.RegisterName("MetricsServer", new(rpcType))
}
