package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// ResourceRegistry is what the catalog needs from a provider schema
// registry (internal/tfschema implements it) to synthesise elements.
type ResourceRegistry interface {
	// Resource returns the settings schema pieces for a Terraform resource type.
	Resource(tfType string) (props map[string]any, required []string, outputs []string, ok bool)
	// Types lists the resource types of a Terraform provider local name.
	Types(local string) []string
	// Has reports whether the provider has a snapshot.
	Has(local string) bool
}

// SetRegistry attaches the provider schema registry.
func (c *Catalog) SetRegistry(r ResourceRegistry) {
	c.registry = r
	c.generated = map[string]*Entry{}
}

// GeneratedID is the element id for a Terraform resource type.
func (c *Catalog) GeneratedID(tfType string) (string, bool) {
	local, _, ok := strings.Cut(tfType, "_")
	if !ok {
		return "", false
	}
	for name, p := range c.Providers {
		if p.LocalName() == local {
			return name + ".res." + tfType, true
		}
	}
	return "", false
}

// IsGeneratedID reports whether an element id denotes a generated element.
func IsGeneratedID(id string) bool { return strings.Contains(id, ".res.") }

// GeneratedSummary is the light listing served to the palette.
type GeneratedSummary struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Resource string `json:"resource"`
	Service  string `json:"service"`
	Category string `json:"category"`
	Icon     string `json:"icon,omitempty"`
}

// Generated lists the generated elements of a catalog provider (aws, gcp, azure).
func (c *Catalog) Generated(provider string) []GeneratedSummary {
	pc, ok := c.Providers[provider]
	if !ok || c.registry == nil || !c.registry.Has(pc.LocalName()) {
		return nil
	}
	types := c.registry.Types(pc.LocalName())
	out := make([]GeneratedSummary, 0, len(types))
	for _, t := range types {
		out = append(out, GeneratedSummary{
			ID: provider + ".res." + t, Label: humanize(t), Resource: t, Service: service(t),
			Category: categoryOf(t), Icon: c.iconFor(provider, t),
		})
	}
	return out
}

func (c *Catalog) generatedEntry(id string) (*Entry, bool) {
	if c.registry == nil {
		return nil, false
	}
	provider, tfType, ok := strings.Cut(id, ".res.")
	if !ok {
		return nil, false
	}
	pc, ok := c.Providers[provider]
	if !ok {
		return nil, false
	}
	c.genMu.Lock()
	defer c.genMu.Unlock()
	if e, ok := c.generated[id]; ok {
		return e, true
	}
	props, required, outputs, ok := c.registry.Resource(tfType)
	if !ok || !strings.HasPrefix(tfType, pc.LocalName()+"_") {
		return nil, false
	}
	// A generated element may live in any container of its provider, so it can
	// be grouped next to the curated elements it references.
	var parents []string
	for _, e := range c.Entries {
		if e.Provider == provider && e.Kind == KindContainer {
			parents = append(parents, e.ID)
		}
	}
	sort.Strings(parents)
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		req := make([]any, len(required))
		for i, r := range required {
			req[i] = r
		}
		schema["required"] = req
	}
	e := &Entry{
		ID: id, Label: humanize(tfType), Provider: provider, Category: categoryOf(tfType),
		Description: fmt.Sprintf("Terraform resource %s, rendered as a plain resource block. Every attribute of the provider schema is available; reference other elements with arrows.", tfType),
		Icon:        c.iconFor(provider, tfType), Kind: KindLeaf, AllowedParents: parents,
		Props: schema, Outputs: outputs, Size: &Size{W: 120, H: 90},
		Terraform: &Terraform{Role: RoleResource, Resource: tfType},
	}
	c.generated[id] = e
	return e, true
}

// humanize turns aws_db_subnet_group into "DB Subnet Group" (provider prefix dropped).
func humanize(tfType string) string {
	_, rest, _ := strings.Cut(tfType, "_")
	words := strings.Split(rest, "_")
	for i, w := range words {
		if up, ok := acronyms[w]; ok {
			words[i] = up
		} else if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

var acronyms = map[string]string{
	"db": "DB", "vpc": "VPC", "ec2": "EC2", "s3": "S3", "iam": "IAM", "acm": "ACM", "alb": "ALB", "lb": "LB", "elb": "ELB", "nat": "NAT", "dns": "DNS",
	"sqs": "SQS", "sns": "SNS", "kms": "KMS", "ecs": "ECS", "eks": "EKS", "ecr": "ECR", "api": "API", "ssm": "SSM", "waf": "WAF", "vpn": "VPN", "ip": "IP",
	"ipam": "IPAM", "cdn": "CDN", "sql": "SQL", "gke": "GKE", "aks": "AKS", "acr": "ACR", "ssl": "SSL", "tls": "TLS", "url": "URL", "id": "ID", "arn": "ARN",
	"ami": "AMI", "ebs": "EBS", "efs": "EFS", "fsx": "FSx", "rds": "RDS", "msk": "MSK", "mq": "MQ", "emr": "EMR", "glue": "Glue", "oidc": "OIDC", "saml": "SAML",
}

func service(tfType string) string {
	_, rest, _ := strings.Cut(tfType, "_")
	svc, _, _ := strings.Cut(rest, "_")
	return svc
}

// categoryOf places a resource type into a palette category by keywords.
func categoryOf(tfType string) string {
	t := tfType
	switch {
	case containsAny(t, "iam", "kms", "secret", "security", "waf", "shield", "guardduty", "inspector", "key_vault", "certificate", "acm", "policy", "role", "firewall", "network_security"):
		return "security"
	case containsAny(t, "lambda", "cloud_run", "cloudfunctions", "function_app", "serverless", "step", "eventbridge", "cloudwatch_event"):
		return "serverless"
	case containsAny(t, "rds", "dynamodb", "db_", "database", "sql", "redshift", "elasticache", "memorystore", "bigtable", "spanner", "firestore", "cosmos", "redis", "neptune", "documentdb", "timestream", "bigquery"):
		return "database"
	case containsAny(t, "s3", "storage", "efs", "fsx", "ebs", "disk", "bucket", "backup", "glacier", "filestore", "registry", "ecr"):
		return "storage"
	case containsAny(t, "vpc", "subnet", "route", "network", "vnet", "gateway", "lb", "load_balancer", "dns", "cloudfront", "cdn", "endpoint", "peering", "vpn", "nat", "eip", "address", "firewall_rule", "front_door", "traffic", "globalaccelerator"):
		return "network"
	case containsAny(t, "sqs", "sns", "pubsub", "eventhub", "servicebus", "kinesis", "mq", "event", "queue", "topic", "dataflow", "glue", "stream"):
		return "integration"
	case containsAny(t, "instance", "ec2", "compute", "ecs", "eks", "kubernetes", "container", "autoscaling", "launch", "batch", "app_service", "virtual_machine", "vm", "node_pool", "gke", "aks", "image", "emr"):
		return "compute"
	}
	return "integration"
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// iconFor reuses a curated icon whose service matches, else the provider's
// generic tile.
func (c *Catalog) iconFor(provider, tfType string) string {
	svc := service(tfType)
	rest := strings.TrimPrefix(tfType, strings.SplitN(tfType, "_", 2)[0]+"_")
	for _, e := range c.Entries {
		if e.Provider != provider || e.Icon == "" || e.Terraform == nil || e.Terraform.Import == nil {
			continue
		}
		res := e.Terraform.Import.Resource
		if res == "" {
			continue
		}
		if res == tfType {
			return e.Icon
		}
		if s2 := service(res); s2 == svc && svc != "" {
			return e.Icon
		}
		_ = rest
	}
	return provider + "/generic.svg"
}
