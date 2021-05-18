package core

import (
	"errors"
	"fmt"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"

	"github.com/valocode/bubbly/events"
)

const (
	ResourceTableName = "_resource"
	SchemaTableName   = "_schema"
	EventTableName    = "_event"
)

// Tasks stores a map of Task by name
type Tasks map[string]Task

// Queries stores a map of Query by name
type Queries map[string]Query

// Conditions stores a map of Condition by name
type Conditions map[string]Condition

// ResourceOutput represents the output from applying a resource
type ResourceOutput struct {
	ID     string
	Status events.Status
	Error  error
	Value  cty.Value
}

// Output returns a cty.Value which can be used inside an HCL EvalContext
// to resolve variables/traversals
func (r *ResourceOutput) Output() cty.Value {
	return cty.ObjectVal(
		map[string]cty.Value{
			"id":     cty.StringVal(r.ID),
			"status": cty.StringVal(r.Status.String()),
			"value":  r.Value,
		},
	)
}

// DataBlocks produces a core.DataBlocks type of this resource.
// We use DataBlocks in place of Data as two Data objects are required to save
// data to the _event table
func (r *ResourceOutput) DataBlocks() (DataBlocks, error) {

	// The resourceID is needed in order to construct the join to the
	// _resource table
	if r.ID == "" {
		return nil, errors.New("unsafe to produce datablocks from ResourceOutput due to missing ID")
	}

	// this data represents the join; how we connect the
	// ResourceOutput to a resource stored in the _resource table
	joinData := Data{
		TableName: ResourceTableName,
		Fields: DataFields{
			"id": cty.StringVal(r.ID),
		},
	}

	// this data represents the data saved to the _event store
	eventData := Data{
		TableName: EventTableName,
		Fields: map[string]cty.Value{
			"status": cty.StringVal(r.Status.String()),
			"time":   cty.StringVal(events.TimeNow()),
			"error":  cty.StringVal(""),
		},
		Joins: []string{ResourceTableName},
	}

	if r.Error != nil {
		eventData.Fields["error"] = cty.StringVal(r.Error.Error())
	}

	return DataBlocks{joinData, eventData}, nil
}

// CriteriaResult defines the output from a criteria.
// There are some cty-->golang-->cty conversions needed, and by defining this
// struct we get some tighter control over that, using tags and gocty.ImpliedType
type CriteriaResult struct {
	Result bool   `cty:"result"`
	Reason string `cty:"reason"`
}

// Value returns a cty.Value for a CriteriaResult.
// No errors should occur. If they do, something is wrong with the struct or
// something really unexpected has happened
func (c CriteriaResult) Value() (cty.Value, error) {
	ty, err := gocty.ImpliedType(c)
	if err != nil {
		return cty.NilVal, fmt.Errorf("unable to get implied type for CriteriaResult: %w", err)
	}
	result, err := gocty.ToCtyValue(c, ty)
	if err != nil {
		return cty.NilVal, fmt.Errorf("error getting cty.Value for CriteriaResult: %w", err)
	}

	return result, nil
}
