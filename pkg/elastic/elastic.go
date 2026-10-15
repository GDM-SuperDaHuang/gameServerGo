package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/olivere/elastic/v7"
)

var (
	//url = "http://119.91.40.66:9200"
	url            = "http://172.16.0.10:9200"
	username       = "elastic"
	password       = "Ninth@elk"
	instance       *elastic.Client
	instanceMu     sync.RWMutex
	uploaderOnce   sync.Once
	uploadQueue    chan uploadTask
	errNameEmpty   = errors.New("name empty")
	errClientNil   = errors.New("instance nil")
	errQueueFull   = errors.New("upload queue full")
	queueSize      = 1024
	requestTimeout = 1 * time.Second
)

type uploadTask struct {
	Index string `json:"index"`
	Body  any    `json:"body"`
}

func init() {
	startUploader()
	InitElastic(url, username, password)
}

func startUploader() {
	uploaderOnce.Do(func() {
		uploadQueue = make(chan uploadTask, queueSize)
		go consumeUploads()
	})
}

func consumeUploads() {
	for task := range uploadQueue {
		go func(t uploadTask) {
			defer func() {
				if r := recover(); r != nil {

				}
			}()
			if err := doUpload(t); err != nil {
				data, _ := json.Marshal(t)
				fmt.Println(fmt.Sprintf("上报异常:%v,data: %s", err, string(data)))
			}
		}(task)
	}
}

func doUpload(task uploadTask) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	_, err := instance.Index().Index(task.Index).BodyJson(map[string]any{
		"@time": time.Now().Format(time.RFC3339),
		"data":  task.Body,
	}).OpType("create").Do(ctx)
	return err
}

func InitElastic(url, username, password string) *elastic.Client {
	if strings.TrimSpace(url) == "" {
		return nil
	}

	esClient, err := elastic.NewClient(
		elastic.SetSniff(false),
		elastic.SetURL(url),
		elastic.SetBasicAuth(username, password))
	if err != nil {
		return nil
	}

	instanceMu.Lock()
	instance = esClient
	instanceMu.Unlock()
	return esClient
}

func Upload(name string, body any) error {
	if strings.TrimSpace(name) == "" {
		return errNameEmpty
	}

	instanceMu.RLock()
	clientNil := instance == nil
	instanceMu.RUnlock()
	if clientNil {
		return errClientNil
	}

	task := uploadTask{
		Index: strings.ToLower(name),
		Body:  body,
	}

	select {
	case uploadQueue <- task:
		return nil
	default:
		return errQueueFull
	}
}
