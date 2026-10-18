package templates

// Template represents an application template available in PDSwarm's App Store.
type Template struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Categories  []string         `json:"categories"`
	Logo        string           `json:"logo"`
	Platform    string           `json:"platform"` // "linux"
	Type        string           `json:"type"`     // "compose" or "container"
	ComposeYAML string           `json:"compose_yaml"`
	Env         []TemplateEnvVar `json:"env,omitempty"`
	Volumes     []string         `json:"volumes,omitempty"`
	Ports       []string         `json:"ports,omitempty"`
	Source      string           `json:"source"` // "curated", "portainer", "casaos"
	Repository  *PortainerRepo   `json:"repository,omitempty"`
}

// TemplateEnvVar represents a configurable environment variable for a template.
type TemplateEnvVar struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Default     string `json:"default"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// DeployTemplateRequest is the payload sent when deploying an app from a template.
type DeployTemplateRequest struct {
	TemplateID  string            `json:"template_id"`
	StackName   string            `json:"stack_name"`
	Env         map[string]string `json:"env"`
	ComposeYAML string            `json:"compose_yaml,omitempty"`
}

// PortainerTemplate represents a template definition from Portainer's templates-2.0.json schema.
type PortainerTemplate struct {
	Type          int               `json:"type"` // 1: container, 2: swarm, 3: compose
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Note          string            `json:"note"`
	Categories    []string          `json:"categories"`
	Platform      string            `json:"platform"`
	Logo          string            `json:"logo"`
	Image         string            `json:"image"`
	Registry      string            `json:"registry"`
	Repository    *PortainerRepo    `json:"repository"`
	Env           []PortainerEnv    `json:"env"`
	Ports         []string          `json:"ports"`
	Volumes       []PortainerVolume `json:"volumes"`
	RestartPolicy string            `json:"restart_policy"`
}

// PortainerRepo defines repository details for a Portainer template.
type PortainerRepo struct {
	URL       string `json:"url"`
	StackFile string `json:"stackfile"`
}

// PortainerEnv defines an environment variable in a Portainer template.
type PortainerEnv struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Default     string `json:"default"`
	Description string `json:"description"`
	Preset      bool   `json:"preset"`
}

// PortainerVolume defines a volume mount in a Portainer template.
type PortainerVolume struct {
	Container string `json:"container"`
	Bind      string `json:"bind"`
	Readonly  bool   `json:"readonly"`
}

// PortainerTemplatesCatalog represents the root structure of Portainer templates JSON.
type PortainerTemplatesCatalog struct {
	Version   string              `json:"version"`
	Templates []PortainerTemplate `json:"templates"`
}

// CasaOSMetadata represents the x-casaos metadata block found in CasaOS compose templates.
type CasaOSMetadata struct {
	Architectures []string          `yaml:"architectures" json:"architectures"`
	Main          string            `yaml:"main" json:"main"`
	Category      string            `yaml:"category" json:"category"`
	Description   map[string]string `yaml:"description" json:"description"`
	Developer     string            `yaml:"developer" json:"developer"`
	Author        string            `yaml:"author" json:"author"`
	Icon          string            `yaml:"icon" json:"icon"`
	Thumbnail     string            `yaml:"thumbnail" json:"thumbnail"`
	Title         map[string]string `yaml:"title" json:"title"`
	Tagline       map[string]string `yaml:"tagline" json:"tagline"`
	PortMap       string            `yaml:"port_map" json:"port_map"`
	Scheme        string            `yaml:"scheme" json:"scheme"`
	Envs          []CasaOSEnv       `yaml:"envs" json:"envs"`
}

// CasaOSEnv defines an environment variable description in CasaOS metadata.
type CasaOSEnv struct {
	Container   string            `yaml:"container" json:"container"`
	Description map[string]string `yaml:"description" json:"description"`
}

