package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	cfncustomresource "github.com/MinneapolisStarTribune/cfn-custom-resource-go"
)

// MathResource demonstrates a generic resource that can customize its
// behavior based on a parameter. Here, it decodes the properties into
// a generic type with only "Op". If Op is "add", it delegates to the
// AdderResource.
func MathResource(r *cfncustomresource.Request) error {
	type GenericProps struct {
		Op string
	}
	gp := &GenericProps{}
	if err := json.Unmarshal(r.ResourceProperties, gp); err != nil {
		return err
	}
	switch gp.Op {
	case "add":
		return AdderResource(r)
	case "":
		return fmt.Errorf("no operation specified")
	default:
		return fmt.Errorf("unimplemented operation %q", gp.Op)
	}
}

// AdderResource adds all integers specified in Addends and returns a
// Sum attribute to the template.
//
// Note that this resource can be used directly, or via MathResource.
func AdderResource(r *cfncustomresource.Request) error {
	if r.RequestType == "Delete" {
		// This resource doesn't create anything, so all deletes succeed
		return r.DeletedResponse().Send()
	}

	type Props struct {
		Op      string
		Addends []int
	}
	type Attrs struct {
		Sum int
	}
	addprops := &Props{}
	if err := json.Unmarshal(r.ResourceProperties, addprops); err != nil {
		return err
	}
	addattrs := &Attrs{} // Sum will be zero-initialized
	for _, v := range addprops.Addends {
		addattrs.Sum += v
	}
	switch r.RequestType {
	case "Create":
		phid := r.RandomPhysicalId(rand.New(rand.NewSource(time.Now().UnixNano())))
		return r.CreatedResponse(phid, addattrs).Send()
	case "Update":
		return r.UpdatedResponse(addattrs).Send()
	case "Delete":
		return r.DeletedResponse().Send()
	}
	panic("invalid request type")
}
