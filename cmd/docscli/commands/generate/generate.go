package generate

import (
	"fmt"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/common-fate/granted-approvals/accesshandler/pkg/providerregistry"
	"github.com/common-fate/granted-approvals/accesshandler/pkg/providers"
	"github.com/common-fate/granted-approvals/accesshandler/pkg/psetup"
	"github.com/common-fate/granted-approvals/pkg/deploy"
	"github.com/common-fate/granted-approvals/pkg/gconfig"
	"gopkg.in/yaml.v3"

	"github.com/urfave/cli/v2"
)

var GenerateCommand = cli.Command{
	Name: "generate",
	Action: func(c *cli.Context) error {
		err := os.RemoveAll("./docs/approvals/providers/registry/")
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
						providerFolder := path.Join("./docs/approvals/providers/registry", providerType)
						uses := fmt.Sprintf("%s@%s", providerType, providerVersion)
						registryTemplateData.Providers = append(registryTemplateData.Providers, RegistryProvider{
							Name: uses,
							Path: path.Join("./", providerType, providerVersion),
						})
						err := os.MkdirAll(providerFolder, os.ModePerm)
						if err != nil {
							return err
						}
						instructions, err := psetup.ParseDocsFS(setuper.SetupDocs(), cfg, psetup.TemplateData{
							AccessHandlerExecutionRoleARN: "{{ Access Handler Execution Role ARN }}",
						})
						if err != nil {
							return err
						}
						providerVersionFile := path.Join(providerFolder, providerVersion+".md")
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
						configYML := new(strings.Builder)
						enc := yaml.NewEncoder(configYML)
						enc.SetIndent(2)
						err = enc.Encode(map[string]deploy.Provider{registeredProvider.DefaultID: {
							Uses: uses,
							With: configMap,
						}})
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
					}
				}
			}
		}
		registryFile := "./docs/approvals/providers/registry/00-provider-registry.md"
		f, err := os.Create(registryFile)
		if err != nil {
			return err
		}
		defer f.Close()
		tmpl, err := template.New("registry").Parse(RegistryTemplate)
		if err != nil {
			return err
		}
		registryPageOutput := new(strings.Builder)
		err = tmpl.ExecuteTemplate(registryPageOutput, "registry", registryTemplateData)
		if err != nil {
			return err
		}
		_, err = f.WriteString(registryPageOutput.String())
		if err != nil {
			return err
		}
		categoryFile := "./docs/approvals/providers/registry/_category_.json"
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

const InstructionTemplate string = `# {{ .Provider }}@{{ .Version }}
:::info
When setting up a provider for your deployment, we recommend using the interactive setup workflow which is available from the Providers tab of your admin dashboard.
:::
## Example granted_deployment.yml Configuration
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
	"position": 1,
	"link": { "type": "doc", "id": "provider-registry" }
  }
  `
