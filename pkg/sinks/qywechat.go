package sinks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/resmoio/kubernetes-event-exporter/pkg/kube"
	"github.com/rs/zerolog/log"
)

type QyWeChatConfig struct {
	Endpoint string                 `yaml:"endpoint"`
	Layout   map[string]interface{} `yaml:"layout"`
	Headers  map[string]string      `yaml:"headers"`
}

func NewQyWeChatSink(cfg *QyWeChatConfig) (Sink, error) {
	return &QyWeChat{cfg: cfg}, nil
}

type QyWeChat struct {
	cfg *QyWeChatConfig
}

func (w *QyWeChat) Close() {
	// No-op
}

func (w *QyWeChat) Send(ctx context.Context, ev *kube.EnhancedEvent) error {
	event, err := serializeEventWithLayout(w.cfg.Layout, ev)
	if err != nil {
		return err
	}

	var eventData map[string]interface{}
	json.Unmarshal([]byte(event), &eventData)
	output := fmt.Sprintf(">Cluster: %s\n>EventType: %s\n>EventNamespace: %s\n>EventKind: %s\n>EventObject: %s\n>EventReason: <font color=\"warning\">%s</font>\n>EventTime: %s\nEventMessage: <font color=\"warning\">%s</font>", eventData["cluster"], eventData["type"], eventData["namespace"], eventData["kind"], eventData["object"], eventData["reason"], eventData["time"], eventData["message"])

	reqBody, err := json.Marshal(map[string]interface{}{
		"msgtype": "markdown",
		// "text":    string([]byte(output)),
		"markdown": map[string]string{"content": string([]byte(output))},
	})
	// log.Debug().Msgf("qywechat req body info: %s", string(reqBody))
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, w.cfg.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	for k, v := range w.cfg.Headers {
		req.Header.Add(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	message := string(body)
	log.Debug().Msgf("qywechat Resp: %s", message)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("not 200: %s", message)
	}
	if strings.Contains(message, "returned HTTP error 429") {
		return fmt.Errorf("rate limited: %s", message)
	}

	return nil
}
