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
	ID         string `json:"id"`
	Label      string `json:"label"`
	Resource   string `json:"resource"`
	Service    string `json:"service"`
	Category   string `json:"category"`
	Icon       string `json:"icon,omitempty"`
	Graphical  bool   `json:"graphical"`  // has an official icon and is a thing of its own
	Attachment bool   `json:"attachment"` // configured inside another element
}

// IsAttachmentType reports whether a Terraform resource type's name alone
// marks it as configuration: the provider "default" resources
// (aws_default_vpc, aws_default_security_group) adopt existing objects and
// are never drawn. Everything else is decided by classify.
func IsAttachmentType(tfType string) bool {
	_, rest, _ := strings.Cut(tfType, "_")
	return strings.HasPrefix(rest, "default_")
}

// AttachmentBinding returns the attribute/output through which an attachment
// type binds to a parent element, if the schemas suggest one.
func (c *Catalog) AttachmentBinding(childID, parentID string) (attr, output string, ok bool) {
	child, cok := c.Get(childID)
	parent, pok := c.Get(parentID)
	if !cok || !pok || child.Terraform == nil || child.Terraform.Role != RoleResource || c.registry == nil {
		return "", "", false
	}
	parentTF := ""
	if parent.Terraform != nil {
		if parent.Terraform.Role == RoleResource {
			parentTF = parent.Terraform.Resource
		} else if parent.Terraform.Import != nil {
			parentTF = parent.Terraform.Import.Resource
		}
	}
	if parentTF == "" {
		return "", "", false
	}
	props, required, _, rok := c.registry.Resource(child.Terraform.Resource)
	if !rok {
		return "", "", false
	}
	related := c.iconKey(child.Provider, child.Terraform.Resource) == c.iconKey(child.Provider, parentTF)
	attr = bindsTo(child.Terraform.Resource, parentTF, props, required, related)
	if attr == "" && strings.HasPrefix(child.Terraform.Resource, parentTF+"_") {
		attr = firstRefAttr(props)
	}
	if attr == "" {
		return "", "", false
	}
	return attr, outputFor(attr, parent), true
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
		icon, _ := c.serviceIcon(provider, t)
		attachment := c.isAttachment(provider, t)
		out = append(out, GeneratedSummary{
			ID: provider + ".res." + t, Label: humanize(t), Resource: t, Service: service(t),
			Category: categoryOf(t), Icon: icon, Graphical: !attachment, Attachment: attachment,
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
	if e, ok := c.generated[id]; ok {
		c.genMu.Unlock()
		return e, true
	}
	c.genMu.Unlock() // isAttachment below takes the lock itself
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
	icon, _ := c.serviceIcon(provider, tfType)
	attachment := c.isAttachment(provider, tfType)
	desc := fmt.Sprintf("Terraform resource %s, rendered as a plain resource block. Every attribute of the provider schema is available; reference other elements with arrows.", tfType)
	if attachment {
		desc = fmt.Sprintf("Terraform resource %s. Configured inside the element it applies to; not drawn on its own.", tfType)
	}
	e := &Entry{
		ID: id, Label: humanize(tfType), Provider: provider, Category: categoryOf(tfType),
		Description: desc, Icon: icon, Kind: KindLeaf, AllowedParents: parents,
		Props: schema, Outputs: outputs, Size: &Size{W: 120, H: 90},
		Terraform:  &Terraform{Role: RoleResource, Resource: tfType},
		Attachment: attachment,
	}
	c.genMu.Lock()
	c.generated[id] = e
	c.genMu.Unlock()
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

// serviceIcon returns the official icon for a resource type (longest matching
// prefix in catalog/icons/<provider>/services.yaml, or the curated element
// importing exactly that type) and whether one exists. Without one the
// element is not graphical.
func (c *Catalog) serviceIcon(provider, tfType string) (string, bool) {
	for _, e := range c.Entries {
		if e.Provider == provider && e.Terraform != nil && e.Terraform.Import != nil && e.Terraform.Import.Resource == tfType && e.Icon != "" {
			return e.Icon, true
		}
	}
	_, rest, _ := strings.Cut(tfType, "_")
	best, bestLen := "", 0
	for prefix, icon := range c.serviceIcons[provider] {
		if (rest == strings.TrimSuffix(prefix, "_") || strings.HasPrefix(rest, prefix) || strings.HasPrefix(rest, prefix+"_")) && len(prefix) > bestLen {
			best, bestLen = icon, len(prefix)
		}
	}
	if best != "" {
		return best, true
	}
	return provider + "/generic.svg", false
}

// iconKey identifies the icon family of a resource type: the services.yaml
// prefix it matches (independent of whether a curated element overrides the
// icon path), or the type itself when nothing matches.
func (c *Catalog) iconKey(provider, tfType string) string {
	_, rest, _ := strings.Cut(tfType, "_")
	best, bestLen := "", 0
	for prefix := range c.serviceIcons[provider] {
		if (rest == strings.TrimSuffix(prefix, "_") || strings.HasPrefix(rest, prefix) || strings.HasPrefix(rest, prefix+"_")) && len(prefix) > bestLen {
			best, bestLen = prefix, len(prefix)
		}
	}
	if best == "" {
		return tfType
	}
	return c.serviceIcons[provider][best] // the icon file: two prefixes may share one icon
}

// AttachmentOption is an attachment type that can be added to a parent, with
// the attribute/output binding that ties it to the parent.
type AttachmentOption struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Resource string `json:"resource"`
	Attr     string `json:"attr"`   // attachment attribute that references the parent
	Output   string `json:"output"` // parent attribute/output referenced
}

// AttachmentsFor lists the attachment types applicable to a parent element,
// derived from the schemas: the type name extends the parent's type, or an
// attribute is named after the parent (bucket, role, topic_arn, subnet_id...).
func (c *Catalog) AttachmentsFor(parentID string) []AttachmentOption {
	pe, ok := c.Get(parentID)
	if !ok || c.registry == nil {
		return nil
	}
	c.genMu.Lock()
	if c.attachCache == nil {
		c.attachCache = map[string][]AttachmentOption{}
	}
	if cached, ok := c.attachCache[parentID]; ok {
		c.genMu.Unlock()
		return cached
	}
	c.genMu.Unlock()

	parentTF := ""
	if pe.Terraform != nil {
		if pe.Terraform.Role == RoleResource {
			parentTF = pe.Terraform.Resource
		} else if pe.Terraform.Import != nil {
			parentTF = pe.Terraform.Import.Resource
		}
	}
	pc, ok := c.Providers[pe.Provider]
	if !ok {
		return nil
	}
	local := pc.LocalName()
	var out []AttachmentOption
	if parentTF != "" {
		for _, t := range c.registry.Types(local) {
			// Only first-level types are drawn; only same-service types attach
			// (an Amazon Connect instance_id is not an EC2 attachment).
			if t == parentTF || !c.isAttachment(pe.Provider, t) || service(t) != service(parentTF) {
				continue
			}
			props, required, _, ok := c.registry.Resource(t)
			if !ok {
				continue
			}
			related := c.iconKey(pe.Provider, t) == c.iconKey(pe.Provider, parentTF)
			attr := bindsTo(t, parentTF, props, required, related)
			if attr == "" && strings.HasPrefix(t, parentTF+"_") {
				// Name extension without an obvious attribute: offer it anyway,
				// bound through the first reference-looking attribute.
				attr = firstRefAttr(props)
			}
			if attr != "" {
				out = append(out, AttachmentOption{ID: pe.Provider + ".res." + t, Label: humanize(t), Resource: t, Attr: attr, Output: outputFor(attr, pe)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	c.genMu.Lock()
	c.attachCache[parentID] = out
	c.genMu.Unlock()
	return out
}

// firstRefAttr returns the first non-container attribute that looks like a
// reference (*_id, *_arn, *_name), or "".
func firstRefAttr(props map[string]any) string {
	names := make([]string, 0, len(props))
	for n := range props {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if containerAttrs[n] {
			continue
		}
		if strings.HasSuffix(n, "_id") || strings.HasSuffix(n, "_arn") || strings.HasSuffix(n, "_name") {
			return n
		}
	}
	return ""
}

// outputFor picks the parent output an attachment attribute should reference.
func outputFor(attr string, parent *Entry) string {
	outs := parent.Outputs
	has := func(n string) bool {
		for _, o := range outs {
			if o == n {
				return true
			}
		}
		if props, ok := parent.Props["properties"].(map[string]any); ok {
			if _, ok := props[n]; ok {
				return true
			}
		}
		return false
	}
	pick := func(suffixes ...string) string {
		for _, sfx := range suffixes {
			for _, o := range outs {
				if o == sfx || strings.HasSuffix(o, "_"+sfx) {
					return o
				}
			}
		}
		return ""
	}
	switch {
	case strings.HasSuffix(attr, "_arn"):
		if o := pick("arn"); o != "" {
			return o
		}
	case strings.HasSuffix(attr, "_url"):
		if o := pick("url"); o != "" {
			return o
		}
	case strings.HasSuffix(attr, "_name") || !strings.Contains(attr, "_id"):
		// bare tokens (bucket, role, function_name, topic) usually take the name
		for _, n := range []string{attr, "name", "bucket"} {
			if has(n) {
				return n
			}
		}
		if o := pick("name"); o != "" {
			return o
		}
	}
	if o := pick("id"); o != "" {
		return o
	}
	return "id"
}
