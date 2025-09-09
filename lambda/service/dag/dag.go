package dag

import "github.com/pennsieve/integration-service/service/models"

type Graph interface {
	GetData() map[string][]string
}

type DAG struct {
	Processors []models.Processor
	Data       map[string][]string
}

func (d *DAG) init() {
	d.Data = make(map[string][]string)
	// Initialize the graph with empty adjacency lists

	for _, processor := range d.Processors {
		// build adjacency list
		dependencies := []string{}
		for _, dependency := range processor.DependsOn {
			dependencies = append(dependencies, dependency.SourceUrl)
		}

		d.Data[processor.SourceUrl] = dependencies
	}
}

func (d *DAG) GetData() map[string][]string {
	d.init()
	return d.Data
}

func NewDAG(processors []models.Processor) Graph {
	return &DAG{processors, nil}
}
