package dag_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/pennsieve/integration-service/service/dag"
	"github.com/pennsieve/integration-service/service/models"
)

func TestDAG(t *testing.T) {
	processors := []models.Processor{
		{
			SourceUrl: "appUrl2",
			DependsOn: []models.ProcessorDependency{
				{SourceUrl: "appUrl1"},
			},
		},
		{
			SourceUrl: "appUrl3",
			DependsOn: []models.ProcessorDependency{
				{SourceUrl: "appUrl2"},
			},
		},
		{
			SourceUrl: "appUrl1",
			DependsOn: []models.ProcessorDependency{},
		},
	}
	dag := dag.NewDAG(processors)
	dag.Init()

	expected := map[string][]string{
		"appUrl1": {},
		"appUrl2": {"appUrl1"},
		"appUrl3": {"appUrl2"},
	}

	got := dag.GetData()
	if !reflect.DeepEqual(expected, got) {
		fmt.Printf("expected %v, got %v", expected, got)
	}
}
