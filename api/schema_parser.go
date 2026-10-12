package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	MaxSchemaBytes  = 64 * 1024 // 64 KB
	MaxSchemaFields = 100
	MaxNestingDepth = 5
)

// ParseAndValidateSchema parses raw schema bytes and validates them against rules and limits.
// It returns a CollectionSchema and a list of path-annotated validation error strings.
func ParseAndValidateSchema(collName string, rawBytes []byte) (*CollectionSchema, []string) {
	if len(rawBytes) > MaxSchemaBytes {
		return nil, []string{fmt.Sprintf("schema exceeds maximum size of 64 KB (got %d bytes)", len(rawBytes))}
	}

	dec := json.NewDecoder(strings.NewReader(string(rawBytes)))
	dec.UseNumber()

	var rawMap map[string]any
	if err := dec.Decode(&rawMap); err != nil {
		return nil, []string{"invalid JSON: " + err.Error()}
	}

	return ValidateRawSchemaMap(collName, rawMap)
}

// ValidateRawSchemaMap validates a parsed JSON map against schema rules and limits.
func ValidateRawSchemaMap(collName string, raw map[string]any) (*CollectionSchema, []string) {
	var errs []string

	name := collName
	if rawName, ok := raw["name"].(string); ok && strings.TrimSpace(rawName) != "" {
		name = strings.TrimSpace(rawName)
	}
	if strings.TrimSpace(name) == "" {
		errs = append(errs, "collection name is required")
	} else if strings.HasPrefix(name, "_") {
		errs = append(errs, "collection names starting with '_' are reserved")
	}

	// Unknown top-level rules check
	for k := range raw {
		switch k {
		case "name", "fields", "additionalFields", "version", "json_schema":
			// allowed
		default:
			errs = append(errs, fmt.Sprintf("%s is an unsupported rule", k))
		}
	}

	additionalFields := "allow"
	if af, ok := raw["additionalFields"]; ok && af != nil {
		if afStr, ok := af.(string); ok {
			if afStr != "allow" && afStr != "reject" {
				errs = append(errs, "additionalFields must be either \"allow\" or \"reject\"")
			} else {
				additionalFields = afStr
			}
		} else {
			errs = append(errs, "additionalFields must be either \"allow\" or \"reject\"")
		}
	}

	var version int
	if vRaw, ok := raw["version"]; ok && vRaw != nil {
		if vf, ok := toFloat(vRaw); ok && vf > 0 {
			version = int(vf)
		}
	}

	fields := make(map[string]FieldSchema)
	fieldsRawAny, hasFields := raw["fields"]
	if hasFields && fieldsRawAny != nil {
		fieldsRaw, ok := fieldsRawAny.(map[string]any)
		if !ok {
			errs = append(errs, "fields must be an object")
		} else {
			totalFields := 0
			parsedFields, fieldErrs := validateFieldsMap("fields", fieldsRaw, 1, &totalFields)
			if len(fieldErrs) > 0 {
				errs = append(errs, fieldErrs...)
			}
			fields = parsedFields
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return &CollectionSchema{
		Name:             name,
		Fields:           fields,
		AdditionalFields: additionalFields,
		Version:          version,
	}, nil
}

func validateFieldsMap(pathPrefix string, raw map[string]any, depth int, totalFields *int) (map[string]FieldSchema, []string) {
	var errs []string
	result := make(map[string]FieldSchema, len(raw))

	if depth > MaxNestingDepth {
		errs = append(errs, fmt.Sprintf("%s: nesting depth exceeds maximum limit of %d", pathPrefix, MaxNestingDepth))
		return result, errs
	}

	for fieldName, fieldVal := range raw {
		*totalFields++
		if *totalFields > MaxSchemaFields {
			errs = append(errs, fmt.Sprintf("schema exceeds maximum limit of %d fields", MaxSchemaFields))
			return result, errs
		}

		fieldPath := pathPrefix + "." + fieldName
		if strings.TrimSpace(fieldName) == "" {
			errs = append(errs, fmt.Sprintf("%s: field name cannot be empty", fieldPath))
			continue
		}
		if strings.HasPrefix(fieldName, "_") || fieldName == "_owner" || fieldName == "_id" {
			errs = append(errs, fmt.Sprintf("%s: names starting with '_' are reserved", fieldPath))
		}

		fieldMap, ok := fieldVal.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s must be an object", fieldPath))
			continue
		}

		cfg, fieldErrs := validateSingleField(fieldPath, fieldMap, depth, totalFields)
		if len(fieldErrs) > 0 {
			errs = append(errs, fieldErrs...)
		}
		result[fieldName] = cfg
	}

	return result, errs
}

func validateSingleField(fieldPath string, raw map[string]any, depth int, totalFields *int) (FieldSchema, []string) {
	var errs []string
	cfg := FieldSchema{}

	// Unknown rules check on this field
	for k := range raw {
		switch k {
		case "type", "required", "default", "enum", "min", "max", "minLength", "maxLength", "properties", "items", "additionalFields":
			// valid rule name
		default:
			errs = append(errs, fmt.Sprintf("%s.%s is an unsupported rule", fieldPath, k))
		}
	}

	// Type check
	typeVal, hasType := raw["type"]
	var typeStr string
	if !hasType || typeVal == nil || typeVal == "" {
		errs = append(errs, fmt.Sprintf("%s.type is required", fieldPath))
	} else {
		if s, ok := typeVal.(string); ok {
			typeStr = s
			switch s {
			case "string", "number", "boolean", "bool", "object", "array":
				cfg.Type = s
			default:
				errs = append(errs, fmt.Sprintf("%s.type has unsupported type %q", fieldPath, s))
			}
		} else {
			errs = append(errs, fmt.Sprintf("%s.type must be a string", fieldPath))
		}
	}

	// Required rule
	if reqVal, ok := raw["required"]; ok && reqVal != nil {
		if r, ok := reqVal.(bool); ok {
			cfg.Required = r
		} else {
			errs = append(errs, fmt.Sprintf("%s.required must be a boolean", fieldPath))
		}
	}

	// Default rule
	if defVal, ok := raw["default"]; ok {
		cfg.Default = defVal
	}

	// AdditionalFields rule (for objects)
	if afVal, ok := raw["additionalFields"]; ok && afVal != nil {
		if af, ok := afVal.(string); ok {
			if af != "allow" && af != "reject" {
				errs = append(errs, fmt.Sprintf("%s.additionalFields must be either \"allow\" or \"reject\"", fieldPath))
			} else {
				cfg.AdditionalFields = af
			}
		} else {
			errs = append(errs, fmt.Sprintf("%s.additionalFields must be either \"allow\" or \"reject\"", fieldPath))
		}
	}

	// Rules type-safety checks
	if typeStr != "" {
		switch typeStr {
		case "string":
			for k := range raw {
				if k == "min" || k == "max" || k == "items" || k == "properties" {
					errs = append(errs, fmt.Sprintf("%s.%s is not allowed for string type", fieldPath, k))
				}
			}
		case "number":
			for k := range raw {
				if k == "minLength" || k == "maxLength" || k == "items" || k == "properties" {
					errs = append(errs, fmt.Sprintf("%s.%s is not allowed for number type", fieldPath, k))
				}
			}
		case "boolean", "bool":
			for k := range raw {
				if k == "min" || k == "max" || k == "minLength" || k == "maxLength" || k == "items" || k == "properties" {
					errs = append(errs, fmt.Sprintf("%s.%s is not allowed for boolean type", fieldPath, k))
				}
			}
		}
	}

	// Min rule
	var minVal *float64
	if minRaw, ok := raw["min"]; ok && minRaw != nil {
		if f, ok := toFloat(minRaw); ok {
			minVal = &f
			cfg.Min = &f
		} else {
			errs = append(errs, fmt.Sprintf("%s.min must be a number", fieldPath))
		}
	}

	// Max rule
	var maxVal *float64
	if maxRaw, ok := raw["max"]; ok && maxRaw != nil {
		if f, ok := toFloat(maxRaw); ok {
			maxVal = &f
			cfg.Max = &f
		} else {
			errs = append(errs, fmt.Sprintf("%s.max must be a number", fieldPath))
		}
	}

	if minVal != nil && maxVal != nil && *minVal > *maxVal {
		errs = append(errs, fmt.Sprintf("%s: min cannot be greater than max", fieldPath))
	}

	// MinLength rule
	var minLenVal *int
	if minLenRaw, ok := raw["minLength"]; ok && minLenRaw != nil {
		if f, ok := toFloat(minLenRaw); ok && f >= 0 && f == float64(int(f)) {
			v := int(f)
			minLenVal = &v
			cfg.MinLength = &v
		} else {
			errs = append(errs, fmt.Sprintf("%s.minLength must be a non-negative integer", fieldPath))
		}
	}

	// MaxLength rule
	var maxLenVal *int
	if maxLenRaw, ok := raw["maxLength"]; ok && maxLenRaw != nil {
		if f, ok := toFloat(maxLenRaw); ok && f >= 0 && f == float64(int(f)) {
			v := int(f)
			maxLenVal = &v
			cfg.MaxLength = &v
		} else {
			errs = append(errs, fmt.Sprintf("%s.maxLength must be a non-negative integer", fieldPath))
		}
	}

	if minLenVal != nil && maxLenVal != nil && *minLenVal > *maxLenVal {
		errs = append(errs, fmt.Sprintf("%s: minLength cannot be greater than maxLength", fieldPath))
	}

	// Enum rule
	if enumRaw, ok := raw["enum"]; ok && enumRaw != nil {
		if arr, ok := enumRaw.([]any); ok {
			if len(arr) == 0 {
				errs = append(errs, fmt.Sprintf("%s.enum must not be empty", fieldPath))
			} else {
				for _, v := range arr {
					if v != nil {
						cfg.Enum = append(cfg.Enum, fmt.Sprint(v))
					}
				}
			}
		} else {
			errs = append(errs, fmt.Sprintf("%s.enum must be an array", fieldPath))
		}
	}

	// Object properties
	if typeStr == "object" {
		propsRaw, hasProps := raw["properties"]
		if !hasProps || propsRaw == nil {
			errs = append(errs, fmt.Sprintf("%s must define properties for object type", fieldPath))
		} else if propsMap, ok := propsRaw.(map[string]any); ok {
			if len(propsMap) == 0 {
				errs = append(errs, fmt.Sprintf("%s.properties must define at least one property", fieldPath))
			} else {
				nestedProps, propErrs := validateFieldsMap(fieldPath+".properties", propsMap, depth+1, totalFields)
				if len(propErrs) > 0 {
					errs = append(errs, propErrs...)
				}
				cfg.Properties = nestedProps
			}
		} else {
			errs = append(errs, fmt.Sprintf("%s.properties must be an object", fieldPath))
		}
	}

	// Array items
	if typeStr == "array" {
		itemsRaw, hasItems := raw["items"]
		if !hasItems || itemsRaw == nil {
			errs = append(errs, fmt.Sprintf("%s must define items for array type", fieldPath))
		} else if itemsMap, ok := itemsRaw.(map[string]any); ok {
			itemCfg, itemErrs := validateSingleField(fieldPath+".items", itemsMap, depth+1, totalFields)
			if len(itemErrs) > 0 {
				errs = append(errs, itemErrs...)
			}
			cfg.Items = &itemCfg
		} else {
			errs = append(errs, fmt.Sprintf("%s.items must be an object", fieldPath))
		}
	}

	return cfg, errs
}
