package cfncustomresource

import "fmt"

// YAMLBool represents a boolean value from a YAML template. This type
// is important because CloudFormation will take a YAML value of true
// and pass it to your resource as a string, which the json unmarshaler
// refuses to parse into a bool. In your attribute types, use YAMLBool.
type YAMLBool bool

// UnmarshalText implements TextUnmarshaler to satisfy json.Unmarshal.
func (yb *YAMLBool) UnmarshalText(b []byte) error {
	switch string(b) {
	case "y", "Y", "yes", "Yes", "YES", "true", "True", "TRUE", "on", "On", "ON":
		*yb = true
	case "n", "N", "no", "No", "NO", "false", "False", "FALSE", "off", "Off", "OFF":
		*yb = false
	default:
		return fmt.Errorf("cannot parse %q as YAML Bool", string(b))
	}
	return nil
}
