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
			SourceUrl: "appUrl1",
			DependsOn: []models.ProcessorDependency{},
		},
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
			SourceUrl: "appUrl4",
			DependsOn: []models.ProcessorDependency{
				{SourceUrl: "appUrl3"},
			},
		},
		{
			SourceUrl: "appUrl5",
			DependsOn: []models.ProcessorDependency{
				{SourceUrl: "appUrl4"}},
		},
	}
	graph := dag.NewDAG(processors)

	expected := map[string][]string{
		"appUrl1": {},
		"appUrl2": {"appUrl1"},
		"appUrl3": {"appUrl2"},
		"appUrl4": {"appUrl3"},
		"appUrl5": {"appUrl4"},
	}

	graphData := graph.GetData()
	if !reflect.DeepEqual(expected, graphData) {
		fmt.Printf("expected data %v, got %v", expected, graphData)
	}

	order, err := dag.TopologicalSortLevels(graphData)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	expectedOrder := [][]string{
		{"appUrl1"},
		{"appUrl2"},
		{"appUrl3"},
		{"appUrl4"},
		{"appUrl5"},
	}

	if !reflect.DeepEqual(expectedOrder, order) {
		t.Errorf("expected order %v, got %v", expectedOrder, order)
	}
}

func TestDAG_Sort(t *testing.T) {
	// simple linear dependencies
	graph := map[string][]string{
		"appUrl1": {},          // No dependencies
		"appUrl2": {"appUrl1"}, // app2 depends on app1
		"appUrl3": {"appUrl2"}, // app3 depends on app2
	}

	order, err := dag.TopologicalSortLevels(graph)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	expected := [][]string{
		{"appUrl1"},
		{"appUrl2"},
		{"appUrl3"},
	}

	if !reflect.DeepEqual(expected, order) {
		t.Errorf("expected %v, got %v", expected, order)
	}

	// parallel processors - more complicated scenario
	graph2 := map[string][]string{
		"appUrl1": {},                     // No dependencies
		"appUrl2": {"appUrl1"},            // app2 depends on app1
		"appUrl3": {"appUrl5", "appUrl2"}, // app3 depends on app2
		"appUrl4": {},
	}

	order, err = dag.TopologicalSortLevels(graph2)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	expected = [][]string{
		{"appUrl1", "appUrl4", "appUrl5"},
		{"appUrl2"},
		{"appUrl3"},
	}

	if !reflect.DeepEqual(expected, order) {
		t.Errorf("expected %v, got %v", expected, order)
	}
}
