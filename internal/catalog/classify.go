package catalog

import (
	"sort"
	"strings"
)

func sortStrings(v []string) { sort.Strings(v) }

// containerAttrs reference where a resource is placed, not what it belongs
// to; they never make a resource an attachment of the container.
var containerAttrs = map[string]bool{
	"vpc_id": true, "subnet_id": true, "subnet_ids": true, "vpc_security_group_ids": true, "availability_zone": true,
	"resource_group_name": true, "resource_group_id": true, "location": true, "subscription_id": true, "virtual_network_name": true, "virtual_network_id": true,
	"project": true, "project_id": true, "network": true, "network_id": true, "subnetwork": true, "subnetwork_id": true, "region": true, "zone": true,
	"folder_id": true, "org_id": true, "organization_id": true, "parent": true, "vpc": true, "subnet": true, "resource_group": true,
}

// isAttachment decides, for a provider's resource type, whether it is drawn
// (first-level) or configured inside another element. Results are cached per
// provider. Never call it while holding genMu.
func (c *Catalog) isAttachment(provider, tfType string) bool {
	c.genMu.Lock()
	if c.attachKinds == nil {
		c.attachKinds = map[string]map[string]bool{}
	}
	kinds, ok := c.attachKinds[provider]
	c.genMu.Unlock()
	if !ok {
		kinds = c.classify(provider)
		c.genMu.Lock()
		c.attachKinds[provider] = kinds
		c.genMu.Unlock()
	}
	return kinds[tfType]
}

// classify computes the attachment flag for every type of a provider.
//
// A type is first-level (drawn, in the palette) unless: (a) it has no
// official icon; (b) its name extends another type's name and that type does
// not itself reference it (aws_s3_bucket_versioning extends aws_s3_bucket;
// but google_sql_database references google_sql_database_instance, so the
// instance stays first-level); or (c) it is owned by a type sharing its icon
// through a binding attribute (aws_lambda_alias.function_name,
// aws_ecs_service.cluster, aws_route.route_table_id) or by any type through
// an attribute named after that type's full name
// (azurerm_storage_container.storage_account_name).
func (c *Catalog) classify(provider string) map[string]bool {
	out := map[string]bool{}
	pc, ok := c.Providers[provider]
	if !ok || c.registry == nil {
		return out
	}
	types := c.registry.Types(pc.LocalName())
	isType := map[string]bool{}
	iconOf := map[string]string{}
	all := make([]string, 0, len(types))
	for _, t := range types {
		isType[t] = true
		icon, official := c.serviceIcon(provider, t)
		if !official || IsAttachmentType(t) {
			out[t] = true
			continue
		}
		iconOf[t] = icon
		all = append(all, t)
	}
	schema := func(t string) (map[string]any, []string) {
		props, required, _, ok := c.registry.Resource(t)
		if !ok {
			return map[string]any{}, nil
		}
		return props, required
	}
	// (b) name extension, unless the shorter type is really the child, or is
	// a container (aws_vpc_endpoint lives in a VPC; it is not VPC configuration).
	containers := map[string]bool{}
	for _, e := range c.Entries {
		if e.Kind == KindContainer && e.Terraform != nil && e.Terraform.Import != nil {
			containers[e.Terraform.Import.Resource] = true
		}
	}
	for _, t := range all {
		ext := extendedType(t, isType)
		if ext == "" || containers[ext] {
			continue
		}
		eprops, ereq := schema(ext)
		if bindsTo(ext, t, eprops, ereq, true) == "" {
			out[t] = true
		}
	}
	// (c) ownership through attributes, within the same service: references
	// across services (a function's KMS key, a VM's image) are edges, not
	// ownership.
	byService := map[string][]string{}
	for _, t := range types {
		byService[service(t)] = append(byService[service(t)], t)
	}
	for _, t := range all {
		if out[t] {
			continue
		}
		props, required := schema(t)
		for _, owner := range byService[service(t)] {
			if owner == t {
				continue
			}
			// Last-word bindings (cluster_name, function_name) only within the
			// same icon family; aws_vpc_endpoint.service_name must not make it a
			// child of aws_vpc_lattice_service.
			related := c.iconKey(provider, owner) == c.iconKey(provider, t)
			if attr := bindsTo(t, owner, props, required, related); attr != "" {
				out[t] = true
				break
			}
		}
	}
	return out
}

// extendedType returns the longest type that t's name extends, or "".
func extendedType(t string, isType map[string]bool) string {
	for i := len(t) - 1; i > 0; i-- {
		if t[i] == '_' && isType[t[:i]] {
			return t[:i]
		}
	}
	return ""
}

// genericWords are attribute names that look like an owner token but are
// plain values (aws_iam_policy.policy is the document, not a reference).
var genericWords = map[string]bool{"policy": true, "name": true, "description": true, "type": true, "key": true, "value": true, "tags": true, "id": true, "arn": true, "role": true, "user": true, "group": true}

// bindsTo returns the attribute through which type t is owned by ownerTF, or
// "". An attribute named after the owner's full type name always counts
// (storage_account_name). When related is true (same icon family), the
// owner's last word(s) count too: bare (cluster, instance, topic) or with a
// required _id/_arn/_name suffix (function_name, target_key_id). Container
// attributes and generic words never count.
func bindsTo(t, ownerTF string, props map[string]any, required []string, related bool) string {
	req := map[string]bool{}
	for _, r := range required {
		req[r] = true
	}
	_, rest, _ := strings.Cut(ownerTF, "_")
	type token struct {
		name     string
		needsReq bool
	}
	tokens := []token{{rest, false}}
	if related {
		words := strings.Split(rest, "_")
		_, tRest, _ := strings.Cut(t, "_")
		tWords := strings.Split(tRest, "_")
		own := tWords[len(tWords)-1] // aws_msk_cluster.cluster_name names itself, not another cluster
		if last := words[len(words)-1]; last != rest && last != own {
			tokens = append(tokens, token{last, true})
		}
		if len(words) > 2 {
			if sub := strings.Join(words[1:], "_"); sub != tRest && !strings.HasSuffix(tRest, "_"+sub) {
				tokens = append(tokens, token{sub, true})
			}
		}
	}
	names := make([]string, 0, len(props))
	for n := range props {
		names = append(names, n)
	}
	sortStrings(names)
	for _, tok := range tokens {
		for _, n := range names {
			if containerAttrs[n] || genericWords[n] {
				continue
			}
			if pm, ok := props[n].(map[string]any); ok && pm["x-json"] == true {
				continue // nested blocks named after another type are not references
			}
			if n == tok.name {
				if configLike(ownerTF) {
					continue // a bare "security_configuration"/"task_definition" is a reference, not ownership
				}
				return n // bare reference: cluster, instance, topic, bucket
			}
			suffixed := n == tok.name+"_id" || n == tok.name+"_arn" || n == tok.name+"_name" || n == tok.name+"_url" || n == tok.name+"_identifier" ||
				strings.HasSuffix(n, "_"+tok.name+"_id") || strings.HasSuffix(n, "_"+tok.name+"_arn") || strings.HasSuffix(n, "_"+tok.name+"_name")
			if suffixed && (!tok.needsReq || req[n]) {
				return n
			}
		}
	}
	return ""
}

// componentWords are the trailing words of owned types that represent
// members running inside a cluster-like owner rather than its configuration.
var componentWords = map[string]bool{"node_group": true, "node_pool": true, "service": true, "instance": true, "replica": true, "broker": true, "worker": true, "node": true, "pool": true, "task_set": true, "instance_group": true, "instance_fleet": true}

// IsClusterType reports whether a first-level type is a cluster-like box that
// holds members: its name ends with _cluster (EKS, ECS, MSK, Aurora, GKE,
// AKS, Redshift, ElastiCache...).
func IsClusterType(tfType string) bool {
	return strings.HasSuffix(tfType, "_cluster") || strings.HasSuffix(tfType, "_replication_group")
}

// componentOwner returns the cluster-like owner when tfType is a member
// component of it, else "".
func (c *Catalog) componentOwner(provider, tfType string) string {
	if !c.isAttachment(provider, tfType) {
		return ""
	}
	owner := c.ownerOf(provider, tfType)
	if owner == "" || !IsClusterType(owner) {
		return ""
	}
	_, rest, _ := strings.Cut(tfType, "_")
	words := strings.Split(rest, "_")
	for i := range words {
		if componentWords[strings.Join(words[i:], "_")] {
			return owner
		}
	}
	return ""
}

// configLike types describe settings rather than hold members; they never own
// other resources (an EMR cluster references its security configuration, it
// does not belong to it).
func configLike(tfType string) bool {
	for _, sfx := range []string{"_configuration", "_config", "_setting", "_settings", "_template", "_definition", "_option_group", "_parameter_group", "_profile", "_schedule"} {
		if strings.HasSuffix(tfType, sfx) {
			return true
		}
	}
	return false
}

// isBoxType reports whether a first-level type is drawn as a box: it is a
// cluster-like owner with at least one member component (EKS with node
// groups, ECS with services, Aurora with cluster instances). Multi-zone
// elements without members (an MSK cluster, a load balancer) stay nodes.
func (c *Catalog) isBoxType(provider, tfType string) bool {
	if !IsClusterType(tfType) || c.isAttachment(provider, tfType) {
		return false
	}
	pc := c.Providers[provider]
	for _, t := range c.registry.Types(pc.LocalName()) {
		if strings.HasPrefix(t, tfType+"_") || service(t) == service(tfType) {
			if c.componentOwner(provider, t) == tfType {
				return true
			}
		}
	}
	return false
}
