package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"flag"

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
	DriverOpts map[string]string `yaml:"driver_opts,omitempty"`
	External   map[string]string `yaml:"external,omitempty"`
	name       map[string]string `yaml:"name,omitempty"`
}

type volume struct {
	Driver, External string
	DriverOpts       map[string]string `yaml:"driver_opts,omitempty"`
}

type service struct {
	ContainerName            string `yaml:"container_name,omitempty"`
	Image                    string
	Networks, Ports, Volumes []string
	Command                  CommandWrapper
	VolumesFrom              []string `yaml:"volumes_from,omitempty"`
	DependsOn                []string `yaml:"depends_on,omitempty"`
	CapAdd                   []string `yaml:"cap_add,omitempty"`
	Build                    BuildWrapper
	Environment              map[string]string
}

// https://docs.docker.com/compose/compose-file/#service-configuration-reference
// command
// Override the default command.

// command: bundle exec thin -p 3000
// The command can also be a list, in a manner similar to dockerfile:

// command: ["bundle", "exec", "thin", "-p", "3000"]

//CommandWrapper handles YAML "command" which has 2 formats
type CommandWrapper struct {
	Command  string
	Commands []string
}

//UnmarshalYAML handles the dynamic parsing of the YAML options
func (w *CommandWrapper) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var err error
	var str string
	if err = unmarshal(&str); err == nil {
		w.Command = str
		return nil
	}

	var commandArray []string
	if err = unmarshal(&commandArray); err == nil {
		w.Commands = commandArray
		return nil
	}
	return nil //TODO: should be an error , something like UNhhandledError
}

// https://docs.docker.com/compose/compose-file/#service-configuration-reference
// build
// Configuration options that are applied at build time.
//
// build can be specified either as a string containing a path to the build context:
// version: '3'
// services:
//   webapp:
//     build: ./dir
//
//Or, as an object with the path specified under context and optionally Dockerfile and args:
// version: '3'
// services:
// 	webapp:
// 	build:
// 		context: ./dir
// 		dockerfile: Dockerfile-alternate
// 		args:
// 		buildno: 1
// If you specify image as well as build, then Compose names the built image with the webapp and optional tag specified in image:
//
// build: ./dir
// image: webapp:tag
// This results in an image named webapp and tagged tag, built from ./dir.

//BuildWrapper handls YAML build which has 2 formats
type BuildWrapper struct {
	BuildString string
	BuildObject map[string]string
}

//UnmarshalYAML handles the dynamic parsing of the YAML options
func (w *BuildWrapper) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var err error
	var buildString string
	if err = unmarshal(&buildString); err == nil {
		//str := command
		//*w = CommandWrapper(str)
		w.BuildString = buildString
		return nil
	}
	// if err != nil {
	// 	return err
	// }
	// return json.Unmarshal([]byte(str), w)

	var buildObject map[string]string
	if err = unmarshal(&buildObject); err == nil {
		//str := command
		//*w = CommandWrapper(commandArray[0])
		w.BuildObject = buildObject
		return nil
	}
	return nil //should be an error , something like UNhhandledError
}

func nodify(s string) string {
	return strings.Replace(s, "-", "_", -1)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

var flagFileOut bool

func init() {
	flag.BoolVar(&flagFileOut, "fileOut", false, "Send Output to a file.")
}

var flagOutputMarkDown bool

func init() {
	flag.BoolVar(&flagOutputMarkDown, "outputMarkDown", false, "Produce MarkDown formatted output.")
}

var flagQuiet bool

func init() {
	flag.BoolVar(&flagQuiet, "quiet", false, "Suppress console output.")
}

var flagHelp bool

func init() {
	flag.BoolVar(&flagHelp, "help", false, "Displays usage Help.")
}

//TODO: implement --help

func main() {
	var (
		bytes            []byte
		err              error
		graph            *gographviz.Graph
		project          string
		cmdlineArguments []string
	)

	if len(os.Args) < 2 {
		log.Fatal("Need at least an input file! USAGE --help")
	}

	if len(os.Args) >= 2 {
		flag.Parse()
		log.Println("Flag --fileOut            :", flagFileOut)
		log.Println("Flag --outputMarkDown     :", flagOutputMarkDown)
		log.Println("Remaining Arguments (tail):", flag.Args())
		cmdlineArguments = flag.Args()
		if flagHelp {
			consoleHelp()
			return
		}
		//log.Fatal("Need at leaset input file!")
	} else {
		cmdlineArguments = os.Args[1:]
	}

	composerFileFullPath, _ := filepath.Abs(cmdlineArguments[0])
	composerFileExtension := filepath.Ext(composerFileFullPath)
	outputFileFullPathBase := strings.Split(composerFileFullPath, composerFileExtension)[0]

	bytes, err = ioutil.ReadFile(cmdlineArguments[0])
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

	if flagFileOut && flagOutputMarkDown {
		fileOutputMarkdown(outputFileFullPathBase, graph.String())
	}
	if !flagQuiet && flagOutputMarkDown {
		consoleOutputMarkdown(graph.String())
	} else if !flagQuiet {
		consoleOutputStandardGraph(graph.String())
	}
}

//consoleHelp responds to cli --help flag
func consoleHelp() {
	const helpmsg = `docker-compose-dot: Generates graph representation of the docker-compose instance
	Command Line <flags> <yaml/yml file> 
		Where <flags> are
		-- fileOut       : Send Output to a file. 
		--outputMarkDown : Produce MarkDown formatted output.
		--quiet          : Suppress console output.
		--help           : Display this help information.

		and <yaml/yml file> is the docker-composer.yaml file.
	`
	log.Print(helpmsg)
}
func consoleOutputStandardGraph(graphString string) {
	// Produce Markdown output with embedded graph
	// graph.String()
	fmt.Print(graphString)
}
func consoleOutputMarkdown(graphString string) {
	// Produce Markdown output with embedded graph
	// graph.String()
	fmt.Print("\n\n```viz\n\n")
	fmt.Print(graphString)
	fmt.Print("```\n\n")
}

func fileOutputMarkdown(outputFileFullPathBase string, graphString string) {
	// Produce Markdown output with embedded graph
	// outputFileFullPathBase
	// graph.String()

	outputMDfileFullPath := outputFileFullPathBase + ".md"
	//pngFile := absName + ".png";

	fout, err := os.Create(outputMDfileFullPath)
	check(err)

	// It's idiomatic to defer a `Close` immediately
	// after opening a file.
	defer fout.Close()

	fmt.Fprintf(fout, "\n```viz\n")
	fmt.Fprintf(fout, graphString)
	fmt.Fprintf(fout, "```\n")

	// Issue a `Sync` to flush writes to stable storage.
	fout.Sync()

}
