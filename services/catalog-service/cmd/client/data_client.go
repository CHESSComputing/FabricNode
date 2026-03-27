
package client

import (
    "bytes"
    "fmt"
    "net/http"
)

func SendTriples(dataServiceURL, beamlineID, did, triples string) error {
    url := fmt.Sprintf("%s/beamlines/%s/datasets/%s/triples", dataServiceURL, beamlineID, did)

    _, err := http.Post(url, "text/plain", bytes.NewBuffer([]byte(triples)))
    return err
}

func Query(dataServiceURL, beamlineID, did, query string) (*http.Response, error) {
    url := fmt.Sprintf("%s/beamlines/%s/datasets/%s/sparql?query=%s",
        dataServiceURL, beamlineID, did, query)

    return http.Get(url)
}
