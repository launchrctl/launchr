package yamldiscovery

import (
	"reflect"

	"github.com/spf13/cobra"

	"github.com/launchrctl/launchr/core/action"
	"github.com/launchrctl/launchr/core/action/jsonschema"
	"github.com/launchrctl/launchr/core/log"
)

func setCobraFlag(ccmd *cobra.Command, opt *action.Option) interface{} {
	var val interface{}
	desc := getCobraCmdDesc(opt.Title, opt.Description)
	switch opt.Type {
	case jsonschema.String:
		val = ccmd.Flags().String(opt.Name, opt.Default.(string), desc)
	case jsonschema.Integer:
		val = ccmd.Flags().Int(opt.Name, opt.Default.(int), desc)
	case jsonschema.Number:
		val = ccmd.Flags().Float64(opt.Name, opt.Default.(float64), desc)
	case jsonschema.Boolean:
		val = ccmd.Flags().Bool(opt.Name, opt.Default.(bool), desc)
	case jsonschema.Array:
		// @todo parse results to requested type somehow
		val = ccmd.Flags().StringSlice(opt.Name, opt.Default.([]string), desc)
	default:
		log.Panic("json schema type %s is not implemented", opt.Type)
	}
	if opt.Required {
		_ = ccmd.MarkFlagRequired(opt.Name)
	}
	return val
}

func derefOpts(opts map[string]interface{}) map[string]interface{} {
	der := make(map[string]interface{})
	for k, v := range opts {
		der[k] = derefOpt(v)
	}
	return der
}

func derefOpt(v interface{}) interface{} {
	switch v := v.(type) {
	case *string:
		return *v
	case *bool:
		return *v
	case *int:
		return *v
	case *float64:
		return *v
	case *[]string:
		return *v
	default:
		if reflect.ValueOf(v).Kind() == reflect.Ptr {
			log.Panic("error on dereferencing value: unsupported %T", v)
		}
		return v
	}
}
