package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	cfncustomresource "github.com/MinneapolisStarTribune/cfn-custom-resource-go"
)

func GreeterResource(r *cfncustomresource.Request) error {
	type Props struct {
		YourName   string
		Salutation string `json:",omitempty"` // optional, default "Hello"
	}
	n := &Props{}
	if err := json.Unmarshal(r.ResourceProperties, n); err != nil {
		return err
	}
	if n.YourName == "" {
		return fmt.Errorf("missing YourName property")
	}
	if n.Salutation == "" {
		n.Salutation = "Hello"
	}
	type Attrs struct {
		Greeting string
	}
	attrs := &Attrs{Greeting: fmt.Sprintf("%s, %s!", n.Salutation, n.YourName)}
	switch r.RequestType {
	case "Create":
		phid := r.RandomPhysicalId(rand.New(rand.NewSource(time.Now().UnixNano())))
		return r.CreatedResponse(phid, attrs).Send()
	case "Update":
		return r.UpdatedResponse(attrs).Send()
	case "Delete":
		return r.DeletedResponse().Send()
	}
	panic("invalid request type")
}

func main() {
	for {
		r := &cfncustomresource.Request{} // from your request handler
		if err := r.Try(GreeterResource); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	}
}
