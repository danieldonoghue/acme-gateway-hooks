package env

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type LookupFunc func(string) (string, bool)

type Validatable interface {
	Validate() error
}

type CommonConfig struct {
	Domain     string `env:"CERTBOT_DOMAIN|ACME_GATEWAY_DOMAIN,required"`
	Validation string `env:"CERTBOT_VALIDATION|ACME_GATEWAY_TOKEN,required"`
	FQDN       string `env:"ACME_GATEWAY_FQDN"`
}

type Option func(*loadOptions)

type loadOptions struct {
	lookup LookupFunc
}

type FieldError struct {
	Field   string
	Message string
}

type LoadError struct {
	Problems []FieldError
}

func (e *LoadError) Error() string {
	if e == nil || len(e.Problems) == 0 {
		return ""
	}
	parts := make([]string, 0, len(e.Problems))
	for _, p := range e.Problems {
		if p.Field == "" {
			parts = append(parts, p.Message)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", p.Field, p.Message))
	}
	return strings.Join(parts, "; ")
}

func WithLookup(lookup LookupFunc) Option {
	return func(o *loadOptions) {
		o.lookup = lookup
	}
}

func Load(dst any, opts ...Option) error {
	if dst == nil {
		return fmt.Errorf("destination must not be nil")
	}

	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("destination must be a non-nil pointer to struct")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("destination must point to a struct")
	}

	options := loadOptions{lookup: os.LookupEnv}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	errs := &LoadError{}
	loadStruct(v, "", options.lookup, errs)

	if len(errs.Problems) > 0 {
		return errs
	}

	return nil
}

func LoadAndValidate(dst any, opts ...Option) error {
	if err := Load(dst, opts...); err != nil {
		return err
	}

	v, ok := dst.(Validatable)
	if !ok {
		return fmt.Errorf("destination must implement Validatable")
	}

	if err := v.Validate(); err != nil {
		return &LoadError{Problems: []FieldError{{Message: err.Error()}}}
	}

	return nil
}

func loadStruct(v reflect.Value, prefix string, lookup LookupFunc, errs *LoadError) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		fv := v.Field(i)

		if sf.PkgPath != "" {
			continue
		}

		fieldName := sf.Name
		if prefix != "" {
			fieldName = prefix + "." + sf.Name
		}

		tag, ok := sf.Tag.Lookup("env")
		if ok && tag == "-" {
			continue
		}

		if !ok {
			if fv.Kind() == reflect.Struct {
				loadStruct(fv, fieldName, lookup, errs)
			}
			continue
		}

		spec, err := parseTag(tag)
		if err != nil {
			errs.Problems = append(errs.Problems, FieldError{Field: fieldName, Message: err.Error()})
			continue
		}

		raw, found := firstNonEmptyLookup(lookup, spec.keys)
		if !found && spec.hasDefault {
			raw = spec.defaultValue
			found = true
		}

		if !found {
			if spec.required {
				errs.Problems = append(errs.Problems, FieldError{Field: fieldName, Message: fmt.Sprintf("missing required environment variable (%s)", strings.Join(spec.keys, " or "))})
			}
			continue
		}

		if err := setFieldValue(fv, raw); err != nil {
			errs.Problems = append(errs.Problems, FieldError{Field: fieldName, Message: err.Error()})
		}
	}
}

type tagSpec struct {
	keys         []string
	required     bool
	hasDefault   bool
	defaultValue string
}

func parseTag(tag string) (tagSpec, error) {
	parts := strings.Split(tag, ",")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return tagSpec{}, fmt.Errorf("invalid env tag")
	}

	keys := strings.Split(parts[0], "|")
	cleanKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k != "" {
			cleanKeys = append(cleanKeys, k)
		}
	}
	if len(cleanKeys) == 0 {
		return tagSpec{}, fmt.Errorf("invalid env tag: no keys defined")
	}

	spec := tagSpec{keys: cleanKeys}
	for _, opt := range parts[1:] {
		opt = strings.TrimSpace(opt)
		switch {
		case opt == "required":
			spec.required = true
		case strings.HasPrefix(opt, "default="):
			spec.hasDefault = true
			spec.defaultValue = strings.TrimSpace(strings.TrimPrefix(opt, "default="))
		case opt == "":
			continue
		default:
			return tagSpec{}, fmt.Errorf("unknown env tag option %q", opt)
		}
	}

	return spec, nil
}

func firstNonEmptyLookup(lookup LookupFunc, keys []string) (string, bool) {
	for _, k := range keys {
		if v, ok := lookup(k); ok {
			v = strings.TrimSpace(v)
			if v != "" {
				return v, true
			}
		}
	}
	return "", false
}

func setFieldValue(field reflect.Value, raw string) error {
	if !field.CanSet() {
		return fmt.Errorf("field is not settable")
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(strings.TrimSpace(raw))
		return nil
	case reflect.Bool:
		parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("invalid boolean value %q: %w", raw, err)
		}
		field.SetBool(parsed)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, int(field.Type().Bits()))
		if err != nil {
			return fmt.Errorf("invalid integer value %q: %w", raw, err)
		}
		field.SetInt(parsed)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(strings.TrimSpace(raw), 10, int(field.Type().Bits()))
		if err != nil {
			return fmt.Errorf("invalid unsigned integer value %q: %w", raw, err)
		}
		field.SetUint(parsed)
		return nil
	default:
		return fmt.Errorf("unsupported field type %s", field.Kind())
	}
}

func (c *CommonConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("common config must not be nil")
	}

	c.Domain = NormalizeFQDN(c.Domain)
	c.Validation = strings.TrimSpace(c.Validation)
	c.FQDN = NormalizeFQDN(c.FQDN)
	if c.FQDN == "" {
		c.FQDN = "_acme-challenge." + c.Domain
	}

	if c.Domain == "" {
		return fmt.Errorf("missing required domain input: CERTBOT_DOMAIN or ACME_GATEWAY_DOMAIN")
	}
	if c.Validation == "" {
		return fmt.Errorf("missing required TXT value input: CERTBOT_VALIDATION or ACME_GATEWAY_TOKEN")
	}

	return nil
}

func NormalizeCommon(c *CommonConfig) {
	_ = c.Validate()
}

func NormalizeFQDN(v string) string {
	return strings.TrimSuffix(strings.TrimSpace(strings.ToLower(v)), ".")
}
