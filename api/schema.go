package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Debjit28/sprig-db/sprig"
)

const schemaCollection = "_schemas"

type FieldSchema struct {
	Type             string                 `json:"type"`
	Required         bool                   `json:"required"`
	Enum             []string               `json:"enum,omitempty"`
	Min              *float64               `json:"min,omitempty"`
	Max              *float64               `json:"max,omitempty"`
	MinLength        *int                   `json:"minLength,omitempty"`
	MaxLength        *int                   `json:"maxLength,omitempty"`
	Default          any                    `json:"default,omitempty"`
	AdditionalFields string                 `json:"additionalFields,omitempty"`

	// For nested validation.
	Properties map[string]FieldSchema `json:"properties,omitempty"` // object
	Items      *FieldSchema           `json:"items,omitempty"`      // array
}

type CollectionSchema struct {
	Name             string                 `json:"name"`
	Fields           map[string]FieldSchema `json:"fields"`
	AdditionalFields string                 `json:"additionalFields,omitempty"`
	Version          int                    `json:"version,omitempty"`
}

func (cs *CollectionSchema) AllowsAdditionalFields() bool {
	return cs.AdditionalFields != "reject"
}

func (cs *CollectionSchema) Validate() error {
	if strings.TrimSpace(cs.Name) == "" {
		return errors.New("collection name is required")
	}
	if strings.HasPrefix(cs.Name, "_") {
		return errors.New("collection names starting with '_' are reserved")
	}
	if cs.AdditionalFields != "" && cs.AdditionalFields != "allow" && cs.AdditionalFields != "reject" {
		return errors.New("additionalFields must be either \"allow\" or \"reject\"")
	}
	if len(cs.Fields) == 0 && cs.AdditionalFields == "reject" {
		return errors.New("schema must define at least one field when additionalFields is reject")
	}

	for field, cfg := range cs.Fields {
		if strings.TrimSpace(field) == "" {
			return errors.New("field name cannot be empty")
		}
		if strings.HasPrefix(field, "_") || field == "_owner" || field == "_id" {
			return fmt.Errorf("field %q: names starting with '_' are reserved", field)
		}
		switch cfg.Type {
		case "string", "number", "bool", "boolean", "object", "array":
		default:
			return fmt.Errorf("field %q has unsupported type %q", field, cfg.Type)
		}
		if cfg.Min != nil && cfg.Max != nil && *cfg.Min > *cfg.Max {
			return fmt.Errorf("field %q has invalid min/max range", field)
		}
		if cfg.MinLength != nil && cfg.MaxLength != nil && *cfg.MinLength > *cfg.MaxLength {
			return fmt.Errorf("field %q has invalid minLength/maxLength range", field)
		}

		switch cfg.Type {
		case "object":
			if len(cfg.Properties) == 0 {
				return fmt.Errorf("field %q must define properties for object type", field)
			}
		case "array":
			if cfg.Items == nil {
				return fmt.Errorf("field %q must define items for array type", field)
			}
		}
	}
	return nil
}

type SchemaStore struct {
	db *sprig.Sprig
}

func NewSchemaStore(db *sprig.Sprig) *SchemaStore {
	return &SchemaStore{db: db}
}

func (s *SchemaStore) Upsert(owner string, schema CollectionSchema) error {
	if err := schema.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(owner) == "" {
		return errors.New("owner is required")
	}

	_ = s.db.CreateCollection(schemaCollection)
	existing, err := s.db.Coll(schemaCollection).Eq(sprig.Map{"name": schema.Name, "owner": owner}).Find()
	if err != nil {
		return err
	}

	version := schema.Version
	if len(existing.Data) > 0 {
		if version <= 0 {
			if oldV, ok := toFloat(existing.Data[0]["version"]); ok && int(oldV) > 0 {
				version = int(oldV) + 1
			} else {
				version = 2
			}
		}
		if err := s.db.Coll(schemaCollection).Eq(sprig.Map{"name": schema.Name, "owner": owner}).Delete(); err != nil {
			return err
		}
	} else if version <= 0 {
		version = 1
	}

	additionalFields := schema.AdditionalFields
	if additionalFields == "" {
		additionalFields = "allow"
	}

	_, err = s.db.Coll(schemaCollection).Insert(sprig.Map{
		"name":             schema.Name,
		"owner":            owner,
		"fields":           schema.Fields,
		"additionalFields": additionalFields,
		"version":          version,
	})
	return err
}

func (s *SchemaStore) Get(owner, collection string) (*CollectionSchema, error) {
	result, err := s.db.Coll(schemaCollection).Eq(sprig.Map{"name": collection, "owner": owner}).Find()
	if err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("schema for collection %q not found", collection)
	}

	row := result.Data[0]
	fieldsRaw, ok := row["fields"].(map[string]any)
	if !ok {
		// Empty fields map is valid for schema-less
		fieldsRaw = map[string]any{}
	}

	fields := make(map[string]FieldSchema, len(fieldsRaw))
	for k, v := range fieldsRaw {
		fieldMap, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("field schema %q is invalid", k)
		}
		cfg, err := parseFieldSchema(fieldMap)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", k, err)
		}
		fields[k] = cfg
	}

	additionalFields, _ := row["additionalFields"].(string)
	if additionalFields == "" {
		additionalFields = "allow"
	}

	version := 1
	if v, ok := toFloat(row["version"]); ok && int(v) > 0 {
		version = int(v)
	}

	return &CollectionSchema{
		Name:             collection,
		Fields:           fields,
		AdditionalFields: additionalFields,
		Version:          version,
	}, nil
}

func (s *SchemaStore) ListByOwner(owner string) ([]string, error) {
	result, err := s.db.Coll(schemaCollection).Eq(sprig.Map{"owner": owner}).Find()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(result.Data))
	for _, row := range result.Data {
		if name, ok := row["name"].(string); ok && name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func parseFieldSchema(raw map[string]any) (FieldSchema, error) {
	cfg := FieldSchema{}

	t, ok := raw["type"].(string)
	if !ok || t == "" {
		return cfg, errors.New("missing type")
	}
	cfg.Type = t

	if required, ok := raw["required"].(bool); ok {
		cfg.Required = required
	}

	if defVal, ok := raw["default"]; ok {
		cfg.Default = defVal
	}

	if addF, ok := raw["additionalFields"].(string); ok {
		cfg.AdditionalFields = addF
	}

	if enumRaw, ok := raw["enum"].([]any); ok {
		for _, v := range enumRaw {
			if s, ok := v.(string); ok {
				cfg.Enum = append(cfg.Enum, s)
			} else if v != nil {
				cfg.Enum = append(cfg.Enum, fmt.Sprint(v))
			}
		}
	}

	if minRaw, ok := toFloat(raw["min"]); ok {
		cfg.Min = &minRaw
	}
	if maxRaw, ok := toFloat(raw["max"]); ok {
		cfg.Max = &maxRaw
	}
	if minLenRaw, ok := toFloat(raw["minLength"]); ok {
		v := int(minLenRaw)
		cfg.MinLength = &v
	}
	if maxLenRaw, ok := toFloat(raw["maxLength"]); ok {
		v := int(maxLenRaw)
		cfg.MaxLength = &v
	}

	if propsRaw, ok := raw["properties"].(map[string]any); ok {
		cfg.Properties = make(map[string]FieldSchema, len(propsRaw))
		for k, v := range propsRaw {
			m, ok := v.(map[string]any)
			if !ok {
				return cfg, fmt.Errorf("property %q is invalid", k)
			}
			nested, err := parseFieldSchema(m)
			if err != nil {
				return cfg, fmt.Errorf("property %q: %w", k, err)
			}
			cfg.Properties[k] = nested
		}
	}

	if itemsRaw, ok := raw["items"].(map[string]any); ok {
		items, err := parseFieldSchema(itemsRaw)
		if err != nil {
			return cfg, fmt.Errorf("items: %w", err)
		}
		cfg.Items = &items
	}
	return cfg, nil
}

func ValidateDocument(doc sprig.Map, schema *CollectionSchema, partial bool) error {
	if schema == nil {
		return errors.New("schema is required")
	}

	// 1. Apply defaults and check required fields
	for field, cfg := range schema.Fields {
		val, exists := doc[field]
		if !exists || val == nil {
			if cfg.Default != nil {
				doc[field] = cfg.Default
				exists = true
				val = cfg.Default
			}
		}
		if !partial && cfg.Required && (!exists || val == nil) {
			return fmt.Errorf("required field %q is missing", field)
		}
	}

	// 2. Validate present fields
	for field, value := range doc {
		if strings.HasPrefix(field, "_") || field == "id" {
			continue
		}
		cfg, ok := schema.Fields[field]
		if !ok {
			if !schema.AllowsAdditionalFields() {
				return fmt.Errorf("field %q is not defined in schema", field)
			}
			continue
		}
		if err := validateField(value, cfg, field); err != nil {
			return err
		}
	}
	return nil
}

func validateField(value any, cfg FieldSchema, field string) error {
	if value == nil {
		if cfg.Required {
			return fmt.Errorf("required field %q is missing", field)
		}
		return nil
	}

	switch cfg.Type {
	case "string":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %q must be string", field)
		}
		if cfg.MinLength != nil && len(s) < *cfg.MinLength {
			return fmt.Errorf("field %q violates minLength", field)
		}
		if cfg.MaxLength != nil && len(s) > *cfg.MaxLength {
			return fmt.Errorf("field %q violates maxLength", field)
		}
		if len(cfg.Enum) > 0 {
			valid := false
			for _, v := range cfg.Enum {
				if s == v {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("field %q is not in enum", field)
			}
		}
	case "number":
		n, ok := toFloat(value)
		if !ok {
			return fmt.Errorf("field %q must be number", field)
		}
		if cfg.Min != nil && n < *cfg.Min {
			return fmt.Errorf("field %q violates min", field)
		}
		if cfg.Max != nil && n > *cfg.Max {
			return fmt.Errorf("field %q violates max", field)
		}
		if len(cfg.Enum) > 0 {
			valid := false
			for _, v := range cfg.Enum {
				enumNum, ok := toFloat(v)
				if (ok && n == enumNum) || fmt.Sprint(n) == v {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("field %q is not in enum", field)
			}
		}
	case "bool", "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field %q must be bool", field)
		}
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("field %q must be object", field)
		}

		// Apply defaults and check required nested fields
		for k, nested := range cfg.Properties {
			v, exists := obj[k]
			if !exists || v == nil {
				if nested.Default != nil {
					obj[k] = nested.Default
					exists = true
				}
			}
			if nested.Required && (!exists || obj[k] == nil) {
				return fmt.Errorf("required field %q is missing", field+"."+k)
			}
		}

		allowAdd := cfg.AdditionalFields != "reject"
		for k := range obj {
			if strings.HasPrefix(k, "_") || k == "id" {
				continue
			}
			if _, ok := cfg.Properties[k]; !ok {
				if !allowAdd {
					return fmt.Errorf("field %q is not defined in schema", field+"."+k)
				}
			}
		}

		// Validate nested values
		for k, nestedCfg := range cfg.Properties {
			if rawVal, ok := obj[k]; ok {
				if err := validateField(rawVal, nestedCfg, field+"."+k); err != nil {
					return err
				}
			}
		}

	case "array":
		arr, ok := value.([]any)
		if !ok {
			return fmt.Errorf("field %q must be array", field)
		}
		if cfg.Items == nil {
			return fmt.Errorf("field %q missing items schema", field)
		}
		for i := range arr {
			if err := validateField(arr[i], *cfg.Items, field+"["+fmt.Sprint(i)+"]"); err != nil {
				return err
			}
		}
	}
	return nil
}

// ConvertJSONSchemaToCollectionSchema converts a (subset of) JSON Schema draft-04
// into this project's CollectionSchema format.
// Supported:
// - object { properties, required }
// - string / number / integer / boolean
// - array { items }
// - enum
// - min/max for number, minLength/maxLength for string
func ConvertJSONSchemaToCollectionSchema(collectionName string, raw map[string]any) (*CollectionSchema, error) {
	if strings.TrimSpace(collectionName) == "" {
		return nil, errors.New("collection name is required")
	}

	topType, _ := raw["type"].(string)
	if topType != "" && topType != "object" {
		return nil, fmt.Errorf("top-level schema type must be object (got %q)", topType)
	}

	propsRaw, ok := raw["properties"].(map[string]any)
	if !ok || len(propsRaw) == 0 {
		return nil, errors.New("json_schema must define properties")
	}

	requiredSet := map[string]bool{}
	if reqRaw, ok := raw["required"].([]any); ok {
		for _, v := range reqRaw {
			if s, ok := v.(string); ok {
				requiredSet[s] = true
			}
		}
	}

	fields := make(map[string]FieldSchema, len(propsRaw))
	for propName, propSchemaRawAny := range propsRaw {
		propSchemaRaw, ok := propSchemaRawAny.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("property %q schema is invalid", propName)
		}
		nestedRequired := requiredSet[propName]
		fieldCfg, err := convertJSONSchemaField(propSchemaRaw, nestedRequired)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", propName, err)
		}
		fields[propName] = fieldCfg
	}

	additionalFields := "allow"
	if af, ok := raw["additionalFields"].(string); ok && af != "" {
		additionalFields = af
	} else if ap, ok := raw["additionalProperties"].(bool); ok && !ap {
		additionalFields = "reject"
	}

	return &CollectionSchema{
		Name:             collectionName,
		Fields:           fields,
		AdditionalFields: additionalFields,
	}, nil
}

func convertJSONSchemaField(raw map[string]any, required bool) (FieldSchema, error) {
	cfg := FieldSchema{Required: required}

	if defVal, ok := raw["default"]; ok {
		cfg.Default = defVal
	}

	t, _ := raw["type"].(string)
	if t == "" {
		return cfg, errors.New("missing type")
	}
	cfg.Type = mapJSONSchemaType(t)

	// enum
	if enumRaw, ok := raw["enum"].([]any); ok {
		for _, v := range enumRaw {
			if s, ok := v.(string); ok {
				cfg.Enum = append(cfg.Enum, s)
			}
		}
	}

	switch cfg.Type {
	case "string":
		if minLenRaw, ok := raw["minLength"].(float64); ok {
			v := int(minLenRaw)
			cfg.MinLength = &v
		}
		if maxLenRaw, ok := raw["maxLength"].(float64); ok {
			v := int(maxLenRaw)
			cfg.MaxLength = &v
		}
	case "number":
		// Accept both JSON schema standard names and our own convenience names.
		if minRaw, ok := raw["minimum"].(float64); ok {
			cfg.Min = &minRaw
		} else if minRaw, ok := raw["min"].(float64); ok {
			cfg.Min = &minRaw
		}
		if maxRaw, ok := raw["maximum"].(float64); ok {
			cfg.Max = &maxRaw
		} else if maxRaw, ok := raw["max"].(float64); ok {
			cfg.Max = &maxRaw
		}
	case "bool", "boolean":
		// no extra constraints
	case "object":
		propsRaw, ok := raw["properties"].(map[string]any)
		if !ok || len(propsRaw) == 0 {
			return cfg, errors.New("object must define properties")
		}
		nestedRequiredSet := map[string]bool{}
		if reqRaw, ok := raw["required"].([]any); ok {
			for _, v := range reqRaw {
				if s, ok := v.(string); ok {
					nestedRequiredSet[s] = true
				}
			}
		}

		cfg.Properties = make(map[string]FieldSchema, len(propsRaw))
		for propName, propSchemaRawAny := range propsRaw {
			propSchemaRaw, ok := propSchemaRawAny.(map[string]any)
			if !ok {
				return cfg, fmt.Errorf("property %q schema is invalid", propName)
			}
			nestedRequired := nestedRequiredSet[propName]
			nestedCfg, err := convertJSONSchemaField(propSchemaRaw, nestedRequired)
			if err != nil {
				return cfg, fmt.Errorf("property %q: %w", propName, err)
			}
			cfg.Properties[propName] = nestedCfg
		}
	case "array":
		itemsRawAny, ok := raw["items"].(map[string]any)
		if !ok {
			return cfg, errors.New("array must define items")
		}
		itemCfg, err := convertJSONSchemaField(itemsRawAny, false)
		if err != nil {
			return cfg, fmt.Errorf("items: %w", err)
		}
		cfg.Items = &itemCfg
	default:
		return cfg, fmt.Errorf("unsupported mapped type %q", cfg.Type)
	}

	return cfg, nil
}

func mapJSONSchemaType(t string) string {
	switch t {
	case "string":
		return "string"
	case "number":
		return "number"
	case "integer":
		return "number"
	case "boolean":
		return "bool"
	case "object":
		return "object"
	case "array":
		return "array"
	default:
		return t
	}
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
