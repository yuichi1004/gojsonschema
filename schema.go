// Copyright 2015 xeipuuv ( https://github.com/xeipuuv )
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// author           xeipuuv
// author-github    https://github.com/xeipuuv
// author-mail      xeipuuv@gmail.com
//
// repository-name  gojsonschema
// repository-desc  An implementation of JSON Schema, based on IETF's draft v4 - Go language.
//
// description      Defines Schema, the main entry to every subSchema.
//                  Contains the parsing logic and error checking.
//
// created          26-02-2013

package gojsonschema

import (
	//	"encoding/json"
	"errors"
	"reflect"
	"regexp"

	"github.com/xeipuuv/gojsonreference"
)

var (
	// Locale is the default locale to use
	// Library users can overwrite with their own implementation
	Locale      locale                    = DefaultLocale{}
	regexpCache map[string]*regexp.Regexp = map[string]*regexp.Regexp{}
)

func regexpCompile(key string) (*regexp.Regexp, error) {
	if re, ok := regexpCache[key]; ok {
		return re, nil
	}

	re, err := regexp.Compile(key)
	if err != nil {
		return nil, err
	}

	regexpCache[key] = re
	return re, nil
}

func NewSchema(l JSONLoader) (*Schema, error) {
	ref, err := l.JsonReference()
	if err != nil {
		return nil, err
	}

	d := Schema{}
	d.pool = newSchemaPool(l.LoaderFactory())
	d.documentReference = ref
	d.referencePool = newSchemaReferencePool()

	var doc interface{}
	if ref.String() != "#" {
		// Get document from schema pool
		spd, err := d.pool.GetDocument(d.documentReference)
		if err != nil {
			return nil, err
		}
		doc = spd.Document
	} else {
		// Load JSON directly
		doc, err = l.LoadJSON()
		if err != nil {
			return nil, err
		}
		d.pool.SetStandaloneDocument(doc)
	}

	err = d.parse(doc)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

type Schema struct {
	documentReference gojsonreference.JsonReference
	rootSchema        *subSchema
	pool              *schemaPool
	referencePool     *schemaReferencePool
}

func (d *Schema) parse(document interface{}) error {
	d.rootSchema = &subSchema{property: STRING_ROOT_SCHEMA_PROPERTY}
	return d.parseSchema(document, d.rootSchema, false)
}

func (d *Schema) SetRootSchemaName(name string) {
	d.rootSchema.property = name
}

// Parses a subSchema
//
// Pretty long function ( sorry :) )... but pretty straight forward, repetitive and boring
// Not much magic involved here, most of the job is to validate the key names and their values,
// then the values are copied into subSchema struct
//
func (d *Schema) parseSchema(documentNode interface{}, currentSchema *subSchema, typeChecked bool) error {

	if !typeChecked && !isKind(documentNode, reflect.Map) {
		return errors.New(formatErrorDescription(
			Locale.InvalidType(),
			ErrorDetails{
				"expected": TYPE_OBJECT,
				"given":    STRING_SCHEMA,
			},
		))
	}

	m := documentNode.(map[string]interface{})

	if currentSchema == d.rootSchema {
		currentSchema.ref = &d.documentReference
	}

	var (
		schemaV,
		refV,
		definitionsV,
		idV,
		titleV,
		descriptionV,
		typeV,
		propsV,
		additionalPropsV,
		patternPropsV,
		dependenciesV,
		itemsV,
		additionalItemsV,
		multipleOfV,
		minimumV,
		exclusiveMinimumV,
		maximumV,
		exclusiveMaximumV,
		minLengthV,
		maxLengthV,
		patternV,
		formatV,
		minPropsV,
		maxPropsV,
		requiredV,
		minItemsV,
		maxItemsV,
		uniqueItemsV,
		enumV,
		oneOfV,
		anyOfV,
		allOfV,
		notV interface{}
	)

	for k, v := range m {
		switch k {
		case KEY_SCHEMA:
			schemaV = v
		case KEY_REF:
			refV = v
		case KEY_DEFINITIONS:
			definitionsV = v
		case KEY_ID:
			idV = v
		case KEY_DESCRIPTION:
			descriptionV = v
		case KEY_TYPE:
			typeV = v
		case KEY_PROPERTIES:
			propsV = v
		case KEY_ADDITIONAL_PROPERTIES:
			additionalPropsV = v
		case KEY_PATTERN_PROPERTIES:
			patternPropsV = v
		case KEY_DEPENDENCIES:
			dependenciesV = v
		case KEY_ITEMS:
			itemsV = v
		case KEY_ADDITIONAL_ITEMS:
			additionalItemsV = v
		case KEY_MULTIPLE_OF:
			multipleOfV = v
		case KEY_MINIMUM:
			minimumV = v
		case KEY_EXCLUSIVE_MINIMUM:
			exclusiveMinimumV = v
		case KEY_MAXIMUM:
			maximumV = v
		case KEY_EXCLUSIVE_MAXIMUM:
			exclusiveMaximumV = v
		case KEY_MIN_LENGTH:
			minLengthV = v
		case KEY_MAX_LENGTH:
			maxLengthV = v
		case KEY_PATTERN:
			patternV = v
		case KEY_FORMAT:
			formatV = v
		case KEY_MIN_PROPERTIES:
			minPropsV = v
		case KEY_MAX_PROPERTIES:
			maxPropsV = v
		case KEY_REQUIRED:
			requiredV = v
		case KEY_MIN_ITEMS:
			minItemsV = v
		case KEY_MAX_ITEMS:
			maxItemsV = v
		case KEY_UNIQUE_ITEMS:
			uniqueItemsV = v
		case KEY_ENUM:
			enumV = v
		case KEY_ONE_OF:
			oneOfV = v
		case KEY_ANY_OF:
			anyOfV = v
		case KEY_ALL_OF:
			allOfV = v
		case KEY_NOT:
			notV = v
		}
	}

	// $subSchema
	if schemaV != nil {
		if schemaRef, ok := schemaV.(string); ok {
			schemaReference, err := gojsonreference.NewJsonReference(schemaRef)
			if err != nil {
				return err
			}
			currentSchema.subSchema = &schemaReference
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": TYPE_STRING,
					"given":    KEY_SCHEMA,
				},
			))
		}
	}

	// $ref
	if refV != nil {
		if k, ok := refV.(string); ok {
			if sch, ok := d.referencePool.Get(currentSchema.ref.String() + k); ok {
				currentSchema.refSchema = sch
			} else {
				return d.parseReference(documentNode, currentSchema, k)
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": TYPE_STRING,
					"given":    KEY_REF,
				},
			))
		}
	}

	// definitions
	if definitionsV != nil {
		if defs, ok := definitionsV.(map[string]interface{}); ok {
			currentSchema.definitions = make(map[string]*subSchema)
			for dk, dv := range defs {
				if isKind(dv, reflect.Map) {
					newSchema := &subSchema{property: KEY_DEFINITIONS, parent: currentSchema, ref: currentSchema.ref}
					currentSchema.definitions[dk] = newSchema
					err := d.parseSchema(dv, newSchema, true)
					if err != nil {
						return errors.New(err.Error())
					}
				} else {
					return errors.New(formatErrorDescription(
						Locale.InvalidType(),
						ErrorDetails{
							"expected": STRING_ARRAY_OF_SCHEMAS,
							"given":    KEY_DEFINITIONS,
						},
					))
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": STRING_ARRAY_OF_SCHEMAS,
					"given":    KEY_DEFINITIONS,
				},
			))
		}
	}

	// id
	if idV != nil {
		if k, ok := idV.(string); ok {
			currentSchema.id = &k
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": TYPE_STRING,
					"given":    KEY_ID,
				},
			))
		}
	}

	// title
	if titleV != nil {
		if k, ok := titleV.(string); ok {
			currentSchema.title = &k
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": TYPE_STRING,
					"given":    KEY_TITLE,
				},
			))
		}
	}

	// description
	if descriptionV != nil {
		if k, ok := descriptionV.(string); ok {
			currentSchema.description = &k
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": TYPE_STRING,
					"given":    KEY_DESCRIPTION,
				},
			))
		}
	}

	// type
	if typeV != nil {
		switch typeV.(type) {
		case string:
			if k, ok := typeV.(string); ok {
				if err := currentSchema.types.Add(k); err != nil {
					return err
				}
			}
		case []interface{}:
			arrayOfTypes := typeV.([]interface{})
			for _, typeInArray := range arrayOfTypes {
				if k, ok := typeInArray.(string); ok {
					currentSchema.types.Add(k)
				} else {
					return errors.New(formatErrorDescription(
						Locale.InvalidType(),
						ErrorDetails{
							"expected": TYPE_STRING + "/" + STRING_ARRAY_OF_STRINGS,
							"given":    KEY_TYPE,
						},
					))
				}
			}
		default:
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": TYPE_STRING + "/" + STRING_ARRAY_OF_STRINGS,
					"given":    KEY_TYPE,
				},
			))
		}
	}

	// properties
	if propsV != nil {
		if err := d.parseProperties(propsV, currentSchema); err != nil {
			return err
		}
	}

	// additionalProperties
	if additionalPropsV != nil {
		switch reflect.ValueOf(additionalPropsV).Kind() {
		case reflect.Bool:
			currentSchema.additionalProperties = additionalPropsV.(bool)
		case reflect.Map:
			newSchema := &subSchema{property: KEY_ADDITIONAL_PROPERTIES, parent: currentSchema, ref: currentSchema.ref}
			currentSchema.additionalProperties = newSchema
			if err := d.parseSchema(additionalPropsV, newSchema, true); err != nil {
				return errors.New(err.Error())
			}
		default:
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": TYPE_BOOLEAN + "/" + STRING_SCHEMA,
					"given":    KEY_ADDITIONAL_PROPERTIES,
				},
			))
		}
	}

	// patternProperties
	if patternPropsV != nil {
		if patternPropertiesMap, ok := patternPropsV.(map[string]interface{}); ok {
			if len(patternPropertiesMap) > 0 {
				currentSchema.patternProperties = make(map[string]*subSchema)
				for k, v := range patternPropertiesMap {
					if _, err := regexpCompile(k); err != nil {
						return errors.New(formatErrorDescription(
							Locale.RegexPattern(),
							ErrorDetails{"pattern": k},
						))
					}

					newSchema := &subSchema{property: k, parent: currentSchema, ref: currentSchema.ref}
					if err := d.parseSchema(v, newSchema, false); err != nil {
						return errors.New(err.Error())
					}
					currentSchema.patternProperties[k] = newSchema
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": STRING_SCHEMA,
					"given":    KEY_PATTERN_PROPERTIES,
				},
			))
		}
	}

	// dependencies
	if dependenciesV != nil {
		if err := d.parseDependencies(dependenciesV, currentSchema); err != nil {
			return err
		}
	}

	// items
	if itemsV != nil {
		switch reflect.ValueOf(itemsV).Kind() {
		case reflect.Slice:
			for _, itemElement := range itemsV.([]interface{}) {
				if isKind(itemElement, reflect.Map) {
					newSchema := &subSchema{parent: currentSchema, property: KEY_ITEMS}
					newSchema.ref = currentSchema.ref
					currentSchema.AddItemsChild(newSchema)
					if err := d.parseSchema(itemElement, newSchema, true); err != nil {
						return err
					}
				} else {
					return errors.New(formatErrorDescription(
						Locale.InvalidType(),
						ErrorDetails{
							"expected": STRING_SCHEMA + "/" + STRING_ARRAY_OF_SCHEMAS,
							"given":    KEY_ITEMS,
						},
					))
				}
				currentSchema.itemsChildrenIsSingleSchema = false
			}
		case reflect.Map:
			newSchema := &subSchema{parent: currentSchema, property: KEY_ITEMS}
			newSchema.ref = currentSchema.ref
			currentSchema.AddItemsChild(newSchema)
			if err := d.parseSchema(itemsV, newSchema, true); err != nil {
				return err
			}
			currentSchema.itemsChildrenIsSingleSchema = true
		default:
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": STRING_SCHEMA + "/" + STRING_ARRAY_OF_SCHEMAS,
					"given":    KEY_ITEMS,
				},
			))
		}
	}

	// additionalItems
	if additionalItemsV != nil {
		switch reflect.ValueOf(additionalItemsV).Kind() {
		case reflect.Bool:
			currentSchema.additionalItems = additionalItemsV.(bool)
		case reflect.Map:
			newSchema := &subSchema{property: KEY_ADDITIONAL_ITEMS, parent: currentSchema, ref: currentSchema.ref}
			currentSchema.additionalItems = newSchema
			if err := d.parseSchema(additionalItemsV, newSchema, true); err != nil {
				return errors.New(err.Error())
			}
		default:
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": TYPE_BOOLEAN + "/" + STRING_SCHEMA,
					"given":    KEY_ADDITIONAL_ITEMS,
				},
			))
		}
	}

	// validation : number / integer
	if multipleOfV != nil {
		multipleOfValue := mustBeNumber(multipleOfV)
		if multipleOfValue == nil {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": STRING_NUMBER,
					"given":    KEY_MULTIPLE_OF,
				},
			))
		}
		if *multipleOfValue <= 0 {
			return errors.New(formatErrorDescription(
				Locale.GreaterThanZero(),
				ErrorDetails{"number": KEY_MULTIPLE_OF},
			))
		}
		currentSchema.multipleOf = multipleOfValue
	}

	if minimumV != nil {
		minimumValue := mustBeNumber(minimumV)
		if minimumValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfA(),
				ErrorDetails{"x": KEY_MINIMUM, "y": STRING_NUMBER},
			))
		}
		currentSchema.minimum = minimumValue
	}

	if exclusiveMinimumV != nil {
		if exclusiveMinimumValue, ok := exclusiveMinimumV.(bool); ok {
			if currentSchema.minimum == nil {
				return errors.New(formatErrorDescription(
					Locale.CannotBeUsedWithout(),
					ErrorDetails{"x": KEY_EXCLUSIVE_MINIMUM, "y": KEY_MINIMUM},
				))
			}
			currentSchema.exclusiveMinimum = exclusiveMinimumValue
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfA(),
				ErrorDetails{"x": KEY_EXCLUSIVE_MINIMUM, "y": TYPE_BOOLEAN},
			))
		}
	}

	if maximumV != nil {
		maximumValue := mustBeNumber(maximumV)
		if maximumValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfA(),
				ErrorDetails{"x": KEY_MAXIMUM, "y": STRING_NUMBER},
			))
		}
		currentSchema.maximum = maximumValue
	}

	if exclusiveMaximumV != nil {
		if exclusiveMaximumValue, ok := exclusiveMaximumV.(bool); ok {
			if currentSchema.maximum == nil {
				return errors.New(formatErrorDescription(
					Locale.CannotBeUsedWithout(),
					ErrorDetails{"x": KEY_EXCLUSIVE_MAXIMUM, "y": KEY_MAXIMUM},
				))
			}
			currentSchema.exclusiveMaximum = exclusiveMaximumValue
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfA(),
				ErrorDetails{"x": KEY_EXCLUSIVE_MAXIMUM, "y": STRING_NUMBER},
			))
		}
	}

	if currentSchema.minimum != nil && currentSchema.maximum != nil {
		if *currentSchema.minimum > *currentSchema.maximum {
			return errors.New(formatErrorDescription(
				Locale.CannotBeGT(),
				ErrorDetails{"x": KEY_MINIMUM, "y": KEY_MAXIMUM},
			))
		}
	}

	// validation : string

	if minLengthV != nil {
		minLengthIntegerValue := mustBeInteger(minLengthV)
		if minLengthIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_MIN_LENGTH, "y": TYPE_INTEGER},
			))
		}
		if *minLengthIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KEY_MIN_LENGTH},
			))
		}
		currentSchema.minLength = minLengthIntegerValue
	}

	if maxLengthV != nil {
		maxLengthIntegerValue := mustBeInteger(maxLengthV)
		if maxLengthIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_MAX_LENGTH, "y": TYPE_INTEGER},
			))
		}
		if *maxLengthIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KEY_MAX_LENGTH},
			))
		}
		currentSchema.maxLength = maxLengthIntegerValue
	}

	if currentSchema.minLength != nil && currentSchema.maxLength != nil {
		if *currentSchema.minLength > *currentSchema.maxLength {
			return errors.New(formatErrorDescription(
				Locale.CannotBeGT(),
				ErrorDetails{"x": KEY_MIN_LENGTH, "y": KEY_MAX_LENGTH},
			))
		}
	}

	if patternV != nil {
		if k, ok := patternV.(string); ok {
			regexpObject, err := regexpCompile(k)
			if err != nil {
				return errors.New(formatErrorDescription(
					Locale.MustBeValidRegex(),
					ErrorDetails{"key": KEY_PATTERN},
				))
			}
			currentSchema.pattern = regexpObject
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfA(),
				ErrorDetails{"x": KEY_PATTERN, "y": TYPE_STRING},
			))
		}
	}

	if formatV != nil {
		formatString, ok := formatV.(string)
		if ok && FormatCheckers.Has(formatString) {
			currentSchema.format = formatString
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeValidFormat(),
				ErrorDetails{"key": KEY_FORMAT, "given": formatV},
			))
		}
	}

	// validation : object

	if minPropsV != nil {
		minPropertiesIntegerValue := mustBeInteger(minPropsV)
		if minPropertiesIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_MIN_PROPERTIES, "y": TYPE_INTEGER},
			))
		}
		if *minPropertiesIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KEY_MIN_PROPERTIES},
			))
		}
		currentSchema.minProperties = minPropertiesIntegerValue
	}

	if maxPropsV != nil {
		maxPropertiesIntegerValue := mustBeInteger(maxPropsV)
		if maxPropertiesIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_MAX_PROPERTIES, "y": TYPE_INTEGER},
			))
		}
		if *maxPropertiesIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KEY_MAX_PROPERTIES},
			))
		}
		currentSchema.maxProperties = maxPropertiesIntegerValue
	}

	if currentSchema.minProperties != nil && currentSchema.maxProperties != nil {
		if *currentSchema.minProperties > *currentSchema.maxProperties {
			return errors.New(formatErrorDescription(
				Locale.KeyCannotBeGreaterThan(),
				ErrorDetails{"key": KEY_MIN_PROPERTIES, "y": KEY_MAX_PROPERTIES},
			))
		}
	}

	if requiredV != nil {
		if requiredValues, ok := requiredV.([]interface{}); ok {
			for _, requiredValue := range requiredValues {
				if k, ok := requiredValue.(string); ok {
					if err := currentSchema.AddRequired(k); err != nil {
						return err
					}
				} else {
					return errors.New(formatErrorDescription(
						Locale.KeyItemsMustBeOfType(),
						ErrorDetails{"key": KEY_REQUIRED, "type": TYPE_STRING},
					))
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_REQUIRED, "y": TYPE_ARRAY},
			))
		}
	}

	// validation : array

	if minItemsV != nil {
		minItemsIntegerValue := mustBeInteger(minItemsV)
		if minItemsIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_MIN_ITEMS, "y": TYPE_INTEGER},
			))
		}
		if *minItemsIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KEY_MIN_ITEMS},
			))
		}
		currentSchema.minItems = minItemsIntegerValue
	}

	if maxItemsV != nil {
		maxItemsIntegerValue := mustBeInteger(maxItemsV)
		if maxItemsIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_MAX_ITEMS, "y": TYPE_INTEGER},
			))
		}
		if *maxItemsIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KEY_MAX_ITEMS},
			))
		}
		currentSchema.maxItems = maxItemsIntegerValue
	}

	if uniqueItemsV != nil {
		if k, ok := uniqueItemsV.(bool); ok {
			currentSchema.uniqueItems = k
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfA(),
				ErrorDetails{"x": KEY_UNIQUE_ITEMS, "y": TYPE_BOOLEAN},
			))
		}
	}

	// validation : all

	if enumV != nil {
		if enumValue, ok := enumV.([]interface{}); ok {
			for _, v := range enumValue {
				if err := currentSchema.AddEnum(v); err != nil {
					return err
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_ENUM, "y": TYPE_ARRAY},
			))
		}
	}

	// validation : subSchema

	if oneOfV != nil {
		if oneOfValue, ok := oneOfV.([]interface{}); ok {
			for _, v := range oneOfValue {
				newSchema := &subSchema{property: KEY_ONE_OF, parent: currentSchema, ref: currentSchema.ref}
				currentSchema.AddOneOf(newSchema)
				if err := d.parseSchema(v, newSchema, false); err != nil {
					return err
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_ONE_OF, "y": TYPE_ARRAY},
			))
		}
	}

	if anyOfV != nil {
		if anyOfValue, ok := anyOfV.([]interface{}); ok {
			for _, v := range anyOfValue {
				newSchema := &subSchema{property: KEY_ANY_OF, parent: currentSchema, ref: currentSchema.ref}
				currentSchema.AddAnyOf(newSchema)
				if err := d.parseSchema(v, newSchema, false); err != nil {
					return err
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_ANY_OF, "y": TYPE_ARRAY},
			))
		}
	}

	if allOfV != nil {
		if allOfValue, ok := allOfV.([]interface{}); ok {
			for _, v := range allOfValue {
				newSchema := &subSchema{property: KEY_ALL_OF, parent: currentSchema, ref: currentSchema.ref}
				currentSchema.AddAllOf(newSchema)
				if err := d.parseSchema(v, newSchema, false); err != nil {
					return err
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_ANY_OF, "y": TYPE_ARRAY},
			))
		}
	}

	if notV != nil {
		if isKind(notV, reflect.Map) {
			newSchema := &subSchema{property: KEY_NOT, parent: currentSchema, ref: currentSchema.ref}
			currentSchema.SetNot(newSchema)
			if err := d.parseSchema(notV, newSchema, true); err != nil {
				return err
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KEY_NOT, "y": TYPE_OBJECT},
			))
		}
	}

	return nil
}

func (d *Schema) parseReference(documentNode interface{}, currentSchema *subSchema, reference string) (e error) {

	var err error

	jsonReference, err := gojsonreference.NewJsonReference(reference)
	if err != nil {
		return err
	}

	standaloneDocument := d.pool.GetStandaloneDocument()

	if jsonReference.HasFullUrl {
		currentSchema.ref = &jsonReference
	} else {
		inheritedReference, err := currentSchema.ref.Inherits(jsonReference)
		if err != nil {
			return err
		}
		currentSchema.ref = inheritedReference
	}

	jsonPointer := currentSchema.ref.GetPointer()

	var refdDocumentNode interface{}

	if standaloneDocument != nil {

		var err error
		refdDocumentNode, _, err = jsonPointer.Get(standaloneDocument)
		if err != nil {
			return err
		}

	} else {

		var err error
		dsp, err := d.pool.GetDocument(*currentSchema.ref)
		if err != nil {
			return err
		}

		refdDocumentNode, _, err = jsonPointer.Get(dsp.Document)
		if err != nil {
			return err
		}

	}

	newSchemaDocument, ok := refdDocumentNode.(map[string]interface{})
	if !ok {
		return errors.New(formatErrorDescription(
			Locale.MustBeOfType(),
			ErrorDetails{"key": STRING_SCHEMA, "type": TYPE_OBJECT},
		))
	}

	// returns the loaded referenced subSchema for the caller to update its current subSchema
	newSchema := &subSchema{property: KEY_REF, parent: currentSchema, ref: currentSchema.ref}
	d.referencePool.Add(currentSchema.ref.String()+reference, newSchema)

	err = d.parseSchema(newSchemaDocument, newSchema, true)
	if err != nil {
		return err
	}

	currentSchema.refSchema = newSchema

	return nil

}

func (d *Schema) parseProperties(documentNode interface{}, currentSchema *subSchema) error {
	m, ok := documentNode.(map[string]interface{})

	if !ok {
		return errors.New(formatErrorDescription(
			Locale.MustBeOfType(),
			ErrorDetails{"key": STRING_PROPERTIES, "type": TYPE_OBJECT},
		))
	}

	for k, v := range m {
		schemaProperty := k
		newSchema := &subSchema{property: schemaProperty, parent: currentSchema, ref: currentSchema.ref}
		currentSchema.AddPropertiesChild(newSchema)
		err := d.parseSchema(v, newSchema, false)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Schema) parseDependencies(documentNode interface{}, currentSchema *subSchema) error {
	m, ok := documentNode.(map[string]interface{})
	if !ok {
		return errors.New(formatErrorDescription(
			Locale.MustBeOfType(),
			ErrorDetails{"key": KEY_DEPENDENCIES, "type": TYPE_OBJECT},
		))
	}

	currentSchema.dependencies = make(map[string]interface{})

	for k, v := range m {
		switch reflect.ValueOf(v).Kind() {

		case reflect.Slice:
			values := v.([]interface{})
			var valuesToRegister []string

			for _, value := range values {
				if val, ok := value.(string); ok {
					valuesToRegister = append(valuesToRegister, val)
				} else {
					return errors.New(formatErrorDescription(
						Locale.MustBeOfType(),
						ErrorDetails{
							"key":  STRING_DEPENDENCY,
							"type": STRING_SCHEMA_OR_ARRAY_OF_STRINGS,
						},
					))
				}
				currentSchema.dependencies[k] = valuesToRegister
			}

		case reflect.Map:
			depSchema := &subSchema{property: k, parent: currentSchema, ref: currentSchema.ref}
			err := d.parseSchema(v, depSchema, true)
			if err != nil {
				return err
			}
			currentSchema.dependencies[k] = depSchema

		default:
			return errors.New(formatErrorDescription(
				Locale.MustBeOfType(),
				ErrorDetails{
					"key":  STRING_DEPENDENCY,
					"type": STRING_SCHEMA_OR_ARRAY_OF_STRINGS,
				},
			))
		}

	}

	return nil
}
