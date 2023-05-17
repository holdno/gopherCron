package warning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/sirupsen/logrus"
)

const (
	ReportHeaderKey      = "Report-Type"
	ReportTypeWarning    = "report_warning"
	ReportTypeTaskResult = "report_task_result"
)

type HttpReporter struct {
	logger        *logrus.Logger
	hc            *http.Client
	reportAddress string
}

func NewHttpReporter(address string) *HttpReporter {
	return &HttpReporter{
		hc: &http.Client{
			Timeout: 5 * time.Second,
		},
		reportAddress: address,
	}
}

func (r *HttpReporter) GetReportAddress() string {
	return r.reportAddress
}

func (r *HttpReporter) Warning(data WarningData) error {
	b, _ := json.Marshal(data)
	req, _ := http.NewRequest(http.MethodPost, r.reportAddress, bytes.NewReader(b))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add(ReportHeaderKey, ReportTypeWarning)

	resp, err := r.hc.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post warning alert, %w", err)
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("warning report failed, log service status error, response status: %d, content: %s",
			resp.StatusCode, string(body))
	}

	return nil
}

func (r *HttpReporter) ResultReport(result *common.TaskExecuteResult) error {
	if result == nil {
		return nil
	}
	b, _ := json.Marshal(result)
	req, _ := http.NewRequest(http.MethodPost, r.reportAddress, bytes.NewReader(b))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add(ReportHeaderKey, ReportTypeTaskResult)

	resp, err := r.hc.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post task result, %w", err)
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("task result report failed, log service status error, response status: %d, content: %s",
			resp.StatusCode, string(body))
	}

	return nil
}
