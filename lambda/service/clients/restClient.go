package clients

import (
	"bytes"
	"io"
	"log"
	"net/http"
)

type ApplicationRestClient struct {
	Client         *http.Client
	ApplicationURL string
}

func NewApplicationRestClient(client *http.Client, url string) Client {
	return &ApplicationRestClient{client, url}
}

func (c *ApplicationRestClient) Execute(b bytes.Buffer) ([]byte, error) {
	req, err := http.NewRequest("POST", c.ApplicationURL, &b)
	// add request headers here
	if err != nil {
		log.Println(err)
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	defer resp.Body.Close()
	s, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return s, err
	}
	return s, nil
}
