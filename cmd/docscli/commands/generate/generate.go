package generate

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/common-fate/common-fate/accesshandler/pkg/providerregistry"
	"github.com/common-fate/common-fate/accesshandler/pkg/providers"
	"github.com/common-fate/common-fate/accesshandler/pkg/psetup"
	"github.com/common-fate/common-fate/pkg/deploy"
	"github.com/common-fate/common-fate/pkg/gconfig"
	"gopkg.in/yaml.v3"

	"github.com/urfave/cli/v3"
)

var GenerateCommand = cli.Command{
	Name:  "generate",
	Flags: []cli.Flag{&cli.StringFlag{Name: "version", Value: "v0.12.0"}},
	Action: func(c *cli.Context) error {
		err := os.RemoveAll("./docs/common-fate/providers/registry/")
		if err != nil {
			return err
		}
		registry := providerregistry.Registry()

		var registryTemplateData RegistryTemplateData

		for providerType, providerVersions := range registry.Providers {
			for providerVersion, registeredProvider := range providerVersions {
				if configer, ok := registeredProvider.Provider.(gconfig.Configer); ok {
					cfg := configer.Config()
					setuper, ok := registeredProvider.Provider.(providers.SetupDocer)
					if ok {

						//update folder structure to be as follows:
						// providers -> registry -> commonfate -> aws-sso -> v2 -> setup
						//												  		-> usage

						providerFolder := path.Join("./docs/common-fate/providers/registry", providerType, providerVersion)
						err := os.MkdirAll(providerFolder, 0700)
						if err != nil {
							return err
						}
						uses := fmt.Sprintf("%s@%s", providerType, providerVersion)
						registryTemplateData.Providers = append(registryTemplateData.Providers, RegistryProvider{
							Name: uses,
							Path: path.Join("./", providerType, providerVersion),
						})

						instructions, err := psetup.ParseDocsFS(setuper.SetupDocs(), cfg, psetup.TemplateData{
							AccessHandlerExecutionRoleARN: "{{ Access Handler Execution Role ARN }}",
						})
						if err != nil {
							return err
						}
						providerVersionFile := path.Join(providerFolder, "setup.md")
						f, err := os.Create(providerVersionFile)
						if err != nil {
							return err
						}
						defer f.Close()

						tmpl, err := template.New("instruction").Parse(InstructionTemplate)
						if err != nil {
							return err
						}

						configMap, err := cfg.Dump(c.Context, gconfig.SafeDumper{})
						if err != nil {
							return err
						}

						// example configuration added for the docs
						deploymentConfig := deploy.Config{
							Version: 2,
							Deployment: deploy.Deployment{
								Release:   c.String("version"),
								StackName: "example",
								Account:   "12345678912",
								Region:    "ap-southeast-2",
								Parameters: deploy.Parameters{
									CognitoDomainPrefix:  "example",
									AdministratorGroupID: "granted_administrators",
									ProviderConfiguration: deploy.ProviderMap{registeredProvider.DefaultID: {
										Uses: uses,
										With: configMap,
									}},
								},
							},
						}

						configYML := new(strings.Builder)
						enc := yaml.NewEncoder(configYML)
						enc.SetIndent(2)
						err = enc.Encode(deploymentConfig)
						if err != nil {
							return err
						}
						instructionData := InstructionTemplateData{
							Steps:            []Step{},
							Provider:         providerType,
							Version:          providerVersion,
							DeploymentConfig: fmt.Sprintf("```yaml\n%s\n```", configYML.String()),
						}
						for _, inst := range instructions {
							step := Step{
								Title:        inst.Title,
								Instructions: inst.Instructions,
								ConfigFields: []ConfigField{},
							}
							for _, field := range inst.ConfigFields {
								step.ConfigFields = append(step.ConfigFields, ConfigField{
									Key:         field.Key(),
									Description: field.Description(),
								})
							}
							instructionData.Steps = append(instructionData.Steps, step)
						}

						instructionsOutput := new(strings.Builder)
						err = tmpl.ExecuteTemplate(instructionsOutput, "instruction", instructionData)
						if err != nil {
							return err
						}
						_, err = f.WriteString(instructionsOutput.String())
						if err != nil {
							return err
						}

						//todo: add this functionality for all providers
						if providerType == "commonfate/aws-sso" {
							usageData := UsageTemplateData{
								Provider: providerType,
								Version:  providerVersion,
							}
							//create the usage file
							providerVersionFileUsage := path.Join(providerFolder, "usage.md")
							f2, err := os.Create(providerVersionFileUsage)
							if err != nil {
								return err
							}
							defer f2.Close()

							//read the usage doc
							//TODO:Update this with a file system read from the common fate repo
							filePath, err := filepath.Abs("./aws-sso-usage/org-units.md")
							if err != nil {
								return err
							}
							data, err := os.ReadFile(filePath)
							if err != nil {
								return err
							}
							usageData.Step = Step{

								Instructions: string(data),
							}

							if err != nil {
								return err
							}

							tmpl, err := template.New("usage").Parse(UsageTemplate)
							if err != nil {
								return err
							}

							usageOutput := new(strings.Builder)
							err = tmpl.ExecuteTemplate(usageOutput, "usage", usageData)
							if err != nil {
								return err
							}
							_, err = f2.WriteString(usageOutput.String())
							if err != nil {
								return err
							}
						}
					}
				}
			}
		}
		registryFile := "./docs/common-fate/providers/registry/00-provider-registry.md"
		f, err := os.Create(registryFile)
		if err != nil {
			return err
		}
		defer f.Close()
		tmpl, err := template.New("registry").Parse(RegistryTemplate)
		if err != nil {
			return err
		}
		sort.Slice(registryTemplateData.Providers, func(i, j int) bool {
			return registryTemplateData.Providers[i].Name < registryTemplateData.Providers[j].Name
		})
		registryPageOutput := new(strings.Builder)
		err = tmpl.ExecuteTemplate(registryPageOutput, "registry", registryTemplateData)
		if err != nil {
			return err
		}
		_, err = f.WriteString(registryPageOutput.String())
		if err != nil {
			return err
		}
		categoryFile := "./docs/common-fate/providers/registry/_category_.json"
		f, err = os.Create(categoryFile)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = f.WriteString(Registry_category_)
		if err != nil {
			return err
		}
		return nil
	},
}

type ConfigField struct {
	Key         string
	Description string
}
type Step struct {
	Title        string
	Instructions string
	ConfigFields []ConfigField
}

type InstructionTemplateData struct {
	Steps            []Step
	Provider         string
	Version          string
	DeploymentConfig string
}

const InstructionTemplate string = `# Setup
## {{ .Provider }}@{{ .Version }}

## Example granted_deployment.yml
{{ .DeploymentConfig }}


{{- range $ix, $option := .Steps}}
## {{ $option.Title }}
### Configuration Fields
This step will guide you through collecting the values for these fields required to setup your provider.

| Field | Description |
| ----------- | ----------- |
{{- range $ix, $field := $option.ConfigFields}}
| {{ $field.Key }} | {{ $field.Description }} |
{{- end}}
{{ $option.Instructions }}
{{- end}}
`

type UsageTemplateData struct {
	Provider string
	Version  string
	Step     Step
}

const UsageTemplate string = `# Usage
## {{ .Provider }}/usage@{{ .Version }}
{{ $.Step.Instructions }}

`

type RegistryProvider struct {
	Name string
	Path string
}
type RegistryTemplateData struct {
	Providers []RegistryProvider
}

const RegistryTemplate string = `---
slug: provider-registry
---

# Provider Registry

Common Fate currently develops a range of providers to manage access to different cloud resources.

{{- range $ix, $provider := .Providers}}

[{{ $provider.Name }}]({{ $provider.Path }})

{{- end}}

Let us know if you have a provider you want added!

We are working toward supporting Community providers which will enable teams to build their own providers for anything such as internal tools.
`

const Registry_category_ string = `{
	"label": "Provider Registry",
	"position": 3,
	"link": { "type": "doc", "id": "provider-registry" }
  }
  `
