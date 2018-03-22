package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"log"

	"github.com/awalterschulze/gographviz"
	yaml "gopkg.in/yaml.v2"
)

type config struct {
	Version  string
	Networks map[string]network
	Volumes  map[string]volume
	Services map[string]service
}

type network struct {
	Driver     string
	DriverOpts map[string]string "driver_opts"
	External   map[string]string "external"
	name       map[string]string "name"
}

type volume struct {
	Driver, External string
	DriverOpts       map[string]string "driver_opts"
}

type service struct {
	ContainerName                     string "container_name"
	Image                             string
	Networks, Ports, Volumes, Command []string
	VolumesFrom                       []string "volumes_from"
	DependsOn                         []string "depends_on"
	CapAdd                            []string "cap_add"
	Build                             struct{ Context, Dockerfile string }
	Environment                       map[string]string
}

func nodify(s string) string {
	return strings.Replace(s, "-", "_", -1)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	var (
		bytes   []byte
		err     error
		graph   *gographviz.Graph
		project string
	)

	if len(os.Args) < 3 {
		log.Fatal("Need input and output file!")
	}

	fout, err := os.Create(os.Args[2])
	check(err)

	// It's idiomatic to defer a `Close` immediately
	// after opening a file.
	defer fout.Close()

	bytes, err = ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	// Parse it as YML
	data := &config{}
	err = yaml.Unmarshal(bytes, &data)
	if err != nil {
		log.Fatal(err)
	}

	// Create directed graph
	graph = gographviz.NewGraph()
	graph.SetName(project)
	graph.SetDir(true)

	// Add legend
	graph.AddSubGraph(project, "cluster_legend", map[string]string{"label": "Legend"})
	graph.AddNode("cluster_legend", "legend_service",
		map[string]string{"shape": "plaintext",
			"label": "<<TABLE BORDER='0'>" +
				"<TR><TD BGCOLOR='lightblue'><B>container_name</B></TD></TR>" +
				"<TR><TD BGCOLOR='lightgrey'><FONT POINT-SIZE='9'>ports ext:int</FONT></TD></TR>" +
				"<TR><TD BGCOLOR='orange'><FONT POINT-SIZE='9'>volumes host:container</FONT></TD></TR>" +
				"<TR><TD BGCOLOR='pink'><FONT POINT-SIZE='9'>environment</FONT></TD></TR>" +
				"</TABLE>>",
		})
	/** NETWORK NODES **/
	for name := range data.Networks {
		/** if external**/
		var ename = name
		if data.Networks[name].External != nil {
			ename = data.Networks[name].External["name"]
		} else {
			ename = name
		}

		graph.AddNode(project, nodify(name), map[string]string{
			"label":     fmt.Sprintf("\"Network: %s\"", ename),
			"style":     "filled",
			"shape":     "box",
			"fillcolor": "palegreen",
		})
	}

	/** SERVICE NODES **/
	for name, service := range data.Services {
		var attrs = map[string]string{"shape": "plaintext", "label": "<<TABLE BORDER='0'>"}
		attrs["label"] += fmt.Sprintf("<TR><TD BGCOLOR='lightblue'><B>%s</B></TD></TR>", name)

		if service.Ports != nil {
			for _, port := range service.Ports {
				attrs["label"] += fmt.Sprintf("<TR><TD BGCOLOR='lightgrey'><FONT POINT-SIZE='9'>%s</FONT></TD></TR>", port)
			}
		}
		if service.Volumes != nil {
			for _, vol := range service.Volumes {
				attrs["label"] += fmt.Sprintf("<TR><TD BGCOLOR='orange'><FONT POINT-SIZE='9'>%s</FONT></TD></TR>", vol)
			}
		}
		/*		if service.Environment != nil {
				for k, v := range service.Environment {
					attrs["label"] += fmt.Sprintf("<TR><TD BGCOLOR='pink'><FONT POINT-SIZE='9'>%s: %s</FONT></TD></TR>",k,v)
				}
			}*/
		attrs["label"] += "</TABLE>>"
		graph.AddNode(project, nodify(name), attrs)
	}
	/** EDGES **/
	for name, service := range data.Services {
		// Links to networks
		if service.Networks != nil {
			for _, linkTo := range service.Networks {
				if strings.Contains(linkTo, ":") {
					linkTo = strings.Split(linkTo, ":")[0]
				}
				graph.AddEdge(nodify(name), nodify(linkTo), true,
					map[string]string{"dir": "none"})
			}
		}
		// volumes_from
		if service.VolumesFrom != nil {
			for _, linkTo := range service.VolumesFrom {
				graph.AddEdge(nodify(name), nodify(linkTo), true,
					map[string]string{"style": "dashed", "label": "volumes_from"})
			}
		}
		// depends_on
		if service.DependsOn != nil {
			for _, linkTo := range service.DependsOn {
				graph.AddEdge(nodify(name), nodify(linkTo), true,
					map[string]string{"style": "dashed", "label": "depends_on"})
			}
		}
	}

	fmt.Fprintf(fout, "\n\n```viz\n\n")
	fmt.Fprintf(fout, graph.String())
	fmt.Fprintf(fout, "```\n\n")

	// Issue a `Sync` to flush writes to stable storage.
	fout.Sync()

}
