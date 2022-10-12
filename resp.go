package cfncustomresource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Response represents the result of processing a Request.
// At the moment, responses must be a maximum of 4096 bytes.
//
// The Resopnse type is of course usable directly, but the most ergonomic
// way to use it is to chain methods together. For instance:
//
//	func SQLRowResourceHandler(r *Request) error {
//		type Props struct {
//			RowValue string
//		}
//		type Attrs struct {
//			Nonce int // a random number generated in the database
//		}
//		p := &Props{}
//		json.Unmarshal(r.ResourceProperties, p) // careful: this example elides some error checking
//		db, _ := sql.Open("postgres", "connection parameters")
//		// this example assumes a table t created as:
//		//   CREATE TABLE t (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), value text, nonce integer);
//		defer db.Close()
//		switch r.RequestType {
//		case "Create":
//			var phid string
//			var nonce int
//			q := `INSERT INTO t (value, nonce) VALUES ($1, trunc(random() * 99999999)) RETURNING id, nonce`
//			if err := db.QueryRowContext(r.Ctx, q, p.RowValue).Scan(&phid, &nonce); err != nil {
//				return err
//			}
//			return r.CreatedResponse(phid, &Attrs{Nonce: nonce}).Send()
//		case "Update":
//			var nonce int
//			q := `UPDATE t SET value=$1 WHERE id=$2 RETURNING nonce`
//			if err := db.QueryRowContext(r.Ctx, q, p.RowValue, r.PhysicalResourceId).Scan(&nonce); err != nil {
//				return err
//			}
//			return r.UpdatedResponse(&Attrs{Nonce: nonce}).Send()
//		case "Delete":
//			q := `DELETE FROM t WHERE id=$1 RETURNING id`
//			var deletedid int // only used to validate at least one row was deleted, otherwise unused
//			if err := db.QueryRowContext(r.Ctx, q, r.PhysicalResourceId).Scan(&deletedid); err != nil {
//				return err
//			}
//			return r.DeletedResponse().Send()
//		}
//		panic("invalid request type")
//	}

type Response struct {
	Status             string
	Reason             string `json:",omitempty"`
	PhysicalResourceId string
	StackId            string
	RequestId          string
	LogicalResourceId  string
	NoEcho             bool        `json:",omitempty"`
	Data               interface{} `json:",omitempty"`

	Ctx context.Context `json:"-"`

	respurl string
	sent    *bool
}

func baseResponse(req *Request) *Response {
	if req.Ctx == nil {
		// because some of our convenience methods rely on Ctx being set,
		// we fill it here if they are being called. req.Ctx will generally
		// be set by our default handler wrapper, though.
		req.Ctx = context.Background()
	}
	return &Response{
		Status:             "SUCCESS",              // may be overridden
		PhysicalResourceId: req.PhysicalResourceId, // may be overridden
		StackId:            req.StackId,            // must be identical
		RequestId:          req.RequestId,          // must be identical
		LogicalResourceId:  req.LogicalResourceId,  // must be identical
		respurl:            req.ResponseURL,        // used internally
		sent:               &req.responseSent,      // used internally
		Ctx:                req.Ctx,                // used internally
	}
}

// Sensitive marks this response as containing potentially sensitive
// values that should not be included in console or API output. This is
// purely advisory, however; values may leak in other ways, such as
// being used as attributes for other resources where masking the value
// is not possible (eg an IAM policy) and so this function is of dubious
// utility. Use with caution. Consider passing sensitive data out of
// band, such as via Systems Manager Parameters or Secrets Manager.
//
// This can be chained for convenience, such as:
//
//	func MyDomainHandler(r *Request) error {
//		type Attrs struct {
//			DomainTransferUnlockKey string
//		}
//		// ... etc ...
//		// Sensitive() returns the same request object
//		return r.UpdatedResponse(&Attrs{DomainTransferUnlockKey: "secret"}).Sensitive().Send()
//	}
func (resp *Response) Sensitive() *Response {
	resp.NoEcho = true
	return resp
}

// Send encodes the Response as a JSON payload and POSTs it to the URL
// provided by CloudFormation in the Request. Note that the response
// payload must be no more than 4096 bytes (per documentation in 2022)
// and so any larger payload will be rejected.
//
// This method is intended to be chained, for example:
//
//	func MyStubHandler(r *Request) error {
//		return r.FailureResponse("not implemented yet").Send()
//	}
func (resp *Response) Send() error {
	body, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("could not marshal Response: %w", err)
	}
	if len(body) > 4096 {
		return fmt.Errorf("response to %q would include payload of %d bytes, exceeds max 4096", resp.respurl, len(body))
	}
	hreq, err := http.NewRequestWithContext(resp.Ctx, "PUT", resp.respurl, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("could not build request object for http callback to %q: %w", resp.respurl, err)
	}
	hreq.ContentLength = int64(len(body))
	result, err := http.DefaultClient.Do(hreq)
	if err != nil {
		return fmt.Errorf("http callback to cloudformation at %q failed: %w", resp.respurl, err)
	}
	if result.StatusCode < 200 || result.StatusCode >= 299 {
		return fmt.Errorf("http callback to %q had unexpected http status code %03d", resp.respurl, result.StatusCode)
	}
	*resp.sent = true // indicate to Request.Try() and friends that we managed to send a Response
	return nil
}
