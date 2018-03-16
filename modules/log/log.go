package log

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"io"
	"gopkg.in/olivere/elastic.v5"
	beegoCtx "github.com/astaxie/beego/context"
	"fmt"
	"time"
)

type ESClient struct {
	client   *elastic.Client
	scrollId string
}

func NewESClient(elasticsearchURL string) (*ESClient, error) {

	if elasticsearchURL == "" {
		elasticsearchURL = os.Getenv("EXTERNAL_ES_URL")
	}
	islog := false
	var options []elastic.ClientOptionFunc
	options = append(options, elastic.SetURL(elasticsearchURL))
	options = append(options, elastic.SetSniff(false))
	if islog {
		errorlog := log.New(os.Stdout, "ELASTICSEARCH ", log.LstdFlags)
		options = append(options, elastic.SetTraceLog(errorlog))
	}
	client, err := elastic.NewClient(options...)
	if err != nil {
		return nil, err
	}

	return &ESClient{client: client}, nil
}

//{"range":{"@timestamp":{"gte":"%s", "lte":"%s"}}}
func (c *ESClient) SearchTodayLog(indexs []string, namespace string, containerName []string, podName, clusterID string, ctx *beegoCtx.Context) error {
	query := elastic.NewBoolQuery()

	var mustQuery []elastic.Query
	mustQuery = make([]elastic.Query, 0)
	mustQuery = append(mustQuery, elastic.NewMatchQuery("kubernetes.namespace_name", namespace))
	mustQuery = append(mustQuery, elastic.NewMatchQuery("kubernetes.pod_name", podName))
	mustQuery = append(mustQuery, elastic.NewMatchQuery("kubernetes.labels.ClusterID", clusterID))
	query.Must(mustQuery...)

	var shouldQuery []elastic.Query
	shouldQuery = make([]elastic.Query, 0)

	if len(containerName) != 0 {
		for _, name := range containerName {
			shouldQuery = append(shouldQuery, elastic.NewMatchQuery("kubernetes.container_name", name))
		}
	}

	query.Should(shouldQuery...)

	svc := c.client.Scroll(indexs...).Query(query).Sort("time_nano", true).Size(200)

	var docs int64 = 0

	for {
		results, err := svc.
		Do(context.Background())
		if err == io.EOF {
			break
		}
		if err != nil && err != io.EOF {
			fmt.Printf("get logs failed from ES:%v\n", err)
			return err
		}

		if results.Hits.TotalHits <= 0 {

			return fmt.Errorf("%s", "no logs in es ")

		}

		for _, hit := range results.Hits.Hits {
			var esHitSource ESHitSource
			data, err := hit.Source.MarshalJSON()
			if err != nil {
				return err
			}
			err = json.Unmarshal(data, &esHitSource)
			if err != nil {
				return err
			}

			if esHitSource.Log != "" {
				ctx.ResponseWriter.Write([]byte(fmt.Sprintf(`<font color="#ffc20e">[%s]</font> %s <br/>`, esHitSource.Timestamp.Add(8 * time.Hour).Format("2006/01/02 15:04:05"), esHitSource.Log)))
			}

		}

		if docs > results.Hits.TotalHits {
			break
		}

		docs = docs + 200

	}

	return nil

}

func (c *ESClient) IndexLogToES(index string, esType string, indexLogData IndexLogData) error {
	indexResult, err := c.client.Index().Index(index).Type(esType).BodyJson(&indexLogData).Do(context.TODO())
	if err != nil {
		return err
	}

	if indexResult == nil {
		return fmt.Errorf("index to ElasticSearch failed")
	}

	return nil

}
