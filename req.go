package cfncustomresource

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
)

// Request represents a CloudFormation request object.
type Request struct {
	RequestType        string // "Create", "Update", or "Delete"
	ResponseURL        string
	StackId            string
	RequestId          string
	ResourceType       string
	LogicalResourceId  string
	ResourceProperties json.RawMessage

	// PhysicalResourceId is only provided with Update and Delete requests. When
	// responding to a Create request, you must provide a PhysicalResourceId for
	// future requests to reference.
	PhysicalResourceId string `json:",omitempty"`

	// OldResourceProperties is only provided with Update requests, and reflects
	// the properties of the resource prior to this update request.
	OldResourceProperties json.RawMessage `json:",omitempty"`

	// Ctx is an optional way to limit runtime for each request
	Ctx context.Context `json:"-"`

	responseSent bool
}

// RandomPhysicalId returns a string suitable for using as a physical id
// if there are no other suitable ids created as a natural consequence
// of the resource. This is particularly useful for virtual resources
// that don't actually create anything.
//
// You must provide a random source. This may be:
//
//	x := r.RandomPhysicalId(rand.New(rand.NewSource(time.Now().UnixNano())))
func (req *Request) RandomPhysicalId(src *rand.Rand) string {
	const chars = "zxcvbnmasdfghjklqwertyuiop1234567890ZXCVBNMASDFGHJKLQWERTYUIOP"
	suffix := make([]byte, 30)
	for j := range suffix {
		suffix[j] = chars[src.Intn(len(chars))]
	}
	return fmt.Sprintf("%s-%s", req.LogicalResourceId, suffix)
}

// A FailureResponse to a request tells CloudFormation that the request
// wasn't able to be completed. In most cases, this will result in a
// stack rollback. A reason must be provided; err.Error() is a good
// place to start.
func (req *Request) FailureResponse(reason string) *Response {
	resp := baseResponse(req)
	resp.Status = "FAILED"
	resp.Reason = reason
	return resp
}

// A CreatedResponse to a request tells CloudFormation that the resource
// was created successfully. A Physical ID must be provided, uniquely
// identifying the created resource. CloudFormation does not inspect
// this value, but does expect it to be unique in an AWS account and
// will provide it with any future update or delete requests. If attr
// is non-nil, its values will be available via !GetAtt in the stack.
func (req *Request) CreatedResponse(physicalid string, attr interface{}) *Response {
	if req.RequestType != "Create" {
		panic("created response on a non-create request")
	}
	if physicalid == "" {
		panic("created response with empty physicalid")
	}
	resp := baseResponse(req)
	resp.PhysicalResourceId = physicalid
	resp.Data = attr
	return resp
}

// A ReplacedResponse to a request tells CloudFormation that a new
// resource was created to satisfy an Update request. A new Physical ID
// must be provided. This will trigger a Delete request for the previous
// Physical ID. If attr is non-nil, its values will be available via
// !GetAtt in the stack.
func (req *Request) ReplacedResponse(physicalid string, attr interface{}) *Response {
	if req.RequestType != "Update" {
		panic("replaced response on a non-update request")
	}
	if req.PhysicalResourceId == physicalid {
		panic("replaced response with same physicalid")
	}
	if physicalid == "" {
		panic("replaced response with empty physicalid")
	}
	resp := baseResponse(req)
	resp.PhysicalResourceId = physicalid
	resp.Data = attr
	return resp
}

// A UpdatedResponse to a request tells CloudFormation that the resource
// was successfully updated in-place. If attr is non-nil, its values
// will be available via !GetAtt in the stack.
func (req *Request) UpdatedResponse(attr interface{}) *Response {
	if req.RequestType != "Update" {
		panic("updated response on a non-update request")
	}
	resp := baseResponse(req)
	resp.Data = attr
	return resp
}

// A DeletedResponse to a request tells CloudFormation that the resource
// was successfully deleted.
func (req *Request) DeletedResponse() *Response {
	if req.RequestType != "Delete" {
		panic("deleted response on a non-delete request")
	}
	return baseResponse(req)
}

// A ReqHandler is a func that processes a single Request and returns
// an error or nil.
type ReqHandler func(*Request) error

// Try runs f with this request, catching panics and returned errors,
// turning them into failure responses. f must call one of the Response
// methods before it returns, or Try will generate a failure.
func (req *Request) Try(f ReqHandler) (err error) {
	// outer defer: inspects err to see if non-nil, and if so, generates
	// and sends a cfn failure response
	defer func() {
		if err != nil {
			if req.responseSent {
				// if a response was already created, just capture and return
				err = fmt.Errorf("received error but response already sent: %w", err)
			} else if ferr := req.FailureResponse(err.Error()).Send(); ferr != nil {
				// something else is wrong, bail out to the runtime
				panic(fmt.Errorf("cannot send error response in error handler: %w", ferr))
			}
		}
	}()

	// inner defer: catches a panic from f and sets err
	defer func() {
		if rec := recover(); rec != nil {
			if perr, ok := rec.(error); ok {
				err = fmt.Errorf("panic with error: %w", perr)
			} else {
				err = fmt.Errorf("panic with value: %v", rec)
			}
		}
	}()

	// run the handler, catching any panic, and capturing any error
	if err = f(req); err != nil {
		return
	}

	if !req.responseSent {
		// f didn't call any Response generating methods
		err = fmt.Errorf("no response generated and no error received")
	}
	return
}
