package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Debjit28/sprig-db/sprig"
)

const schemaCollection = "_schemas"

type FieldSchema struct {
	Type      string   `json:"type"`
	Required  bool     `json:"required"`
	Enum      []string `json:"enum,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`

	// For nested validation.
	Properties map[string]FieldSchema `json:"properties,omitempty"` // object
	Items      *FieldSchema           `json:"items,omitempty"`      // array
}

type CollectionSchema struct {
	Name   string                 `json:"name"`
	Fields map[string]FieldSchema `json:"fields"`
}

func (cs *CollectionSchema) Validate() error {
	if strings.TrimSpace(cs.Name) == "" {
		return errors.New("collection name is required")
	}
	if strings.HasPrefix(cs.Name, "_") {
		return errors.New("collection names starting with '_' are reserved")
	}
	if len(cs.Fields) == 0 {
		return errors.New("schema must define at least one field")
	}

	for field, cfg := range cs.Fields {
		if strings.TrimSpace(field) == "" {
			return errors.New("field name cannot be empty")
		}
		switch cfg.Type {
		case "string", "number", "bool", "object", "array":
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
	if len(existing.Data) > 0 {
		if err := s.db.Coll(schemaCollection).Eq(sprig.Map{"name": schema.Name, "owner": owner}).Delete(); err != nil {
			return err
		}
	}

	_, err = s.db.Coll(schemaCollection).Insert(sprig.Map{
		"name":   schema.Name,
		"owner":  owner,
		"fields": schema.Fields,
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
		return nil, fmt.Errorf("schema for collection %q is corrupted", collection)
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

	return &CollectionSchema{
		Name:   collection,
		Fields: fields,
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

	if enumRaw, ok := raw["enum"].([]any); ok {
		for _, v := range enumRaw {
			if s, ok := v.(string); ok {
				cfg.Enum = append(cfg.Enum, s)
			}
		}
	}

	if minRaw, ok := raw["min"].(float64); ok {
		cfg.Min = &minRaw
	}
	if maxRaw, ok := raw["max"].(float64); ok {
		cfg.Max = &maxRaw
	}
	if minLenRaw, ok := raw["minLength"].(float64); ok {
		v := int(minLenRaw)
		cfg.MinLength = &v
	}
	if maxLenRaw, ok := raw["maxLength"].(float64); ok {
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

	if !partial {
		for field, cfg := range schema.Fields {
			if cfg.Required {
				if _, ok := doc[field]; !ok {
					return fmt.Errorf("required field %q is missing", field)
				}
			}
		}
	}

	for field, value := range doc {
		cfg, ok := schema.Fields[field]
		if !ok {
			return fmt.Errorf("field %q is not defined in schema", field)
		}
		if err := validateField(value, cfg, field); err != nil {
			return err
		}
	}
	return nil
}

func validateField(value any, cfg FieldSchema, field string) error {
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
	case "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field %q must be bool", field)
		}
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			// JSON can decode into map[string]interface{} as map[string]any; keep error simple.
			return fmt.Errorf("field %q must be object", field)
		}

		// Required nested fields.
		for k, nested := range cfg.Properties {
			if nested.Required {
				if _, ok := obj[k]; !ok {
					return fmt.Errorf("required field %q is missing", field+"."+k)
				}
			}
		}

		// Strict unknown field rejection for objects.
		for k := range obj {
			if _, ok := cfg.Properties[k]; !ok {
				return fmt.Errorf("field %q is not defined in schema", field+"."+k)
			}
		}

		// Validate nested values.
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

	return &CollectionSchema{
		Name:   collectionName,
		Fields: fields,
	}, nil
}

func convertJSONSchemaField(raw map[string]any, required bool) (FieldSchema, error) {
	cfg := FieldSchema{Required: required}

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
	case "bool":
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
	default:
		return 0, false
	}
}
