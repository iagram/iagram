package catalog

import (
	"path"
	"sort"
	"strings"
)

// Family is one palette entry of a provider: an official icon and the
// first-level resource types that share it. Dropping a family creates its
// default type; the settings panel lets the user switch to a sibling type.
type Family struct {
	Key      string   `json:"key"`               // icon path, unique per provider
	Label    string   `json:"label"`             // service name derived from the icon
	Icon     string   `json:"icon"`              // icon path
	Category string   `json:"category"`          // palette group
	Types    []string `json:"types"`             // first-level element ids, default first
	Default  string   `json:"default"`           // element id created on drop
	Curated  string   `json:"curated,omitempty"` // curated element id using this icon, if any
}

// Families lists the palette entries of a provider: one per official icon,
// built from the first-level generated types. Icons a curated element already
// uses are reported with Curated set so the palette shows the curated element.
func (c *Catalog) Families(provider string) []Family {
	pc, ok := c.Providers[provider]
	if !ok || c.registry == nil {
		return nil
	}
	// Group by drawn icon (resource icon when one is mapped, else the service
	// icon), so a curated element and the generated types sharing its icon
	// land in one family.
	curatedByKey := map[string]string{}
	curatedIcon := map[string]string{}
	for _, e := range c.Entries {
		if e.Provider == provider && e.Icon != "" && e.Terraform != nil && e.Terraform.Import != nil && e.Terraform.Import.Resource != "" {
			k, _ := c.serviceIcon(provider, e.Terraform.Import.Resource)
			curatedByKey[k] = e.ID
			curatedIcon[k] = e.Icon
		}
	}
	defaults := c.familyDefaults(provider)
	// Owners get the default slot: count how many types bind to each.
	local := pc.LocalName()
	types := c.registry.Types(local)
	owned := map[string]int{}
	byIcon := map[string][]string{}
	for _, t := range types {
		if c.isAttachment(provider, t) {
			if o := c.ownerOf(provider, t); o != "" {
				owned[o]++
			}
			continue
		}
		key, official := c.serviceIcon(provider, t)
		if !official {
			continue
		}
		byIcon[key] = append(byIcon[key], t)
	}
	var out []Family
	for key, list := range byIcon {
		preferred := defaults[key]
		sort.Slice(list, func(i, j int) bool {
			if (list[i] == preferred) != (list[j] == preferred) {
				return list[i] == preferred
			}
			if owned[list[i]] != owned[list[j]] {
				return owned[list[i]] > owned[list[j]]
			}
			if len(list[i]) != len(list[j]) {
				return len(list[i]) < len(list[j])
			}
			return list[i] < list[j]
		})
		ids := make([]string, len(list))
		for i, t := range list {
			ids[i] = provider + ".res." + t
		}
		icon := key
		if ci, ok := curatedIcon[key]; ok {
			icon = ci
		}
		label := c.serviceLabels[provider][key]
		if label == "" {
			label = familyLabel(key)
		}
		f := Family{Key: key, Label: label, Icon: icon, Category: categoryOf(list[0]), Types: ids, Default: ids[0], Curated: curatedByKey[key]}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// familyDefaults reads the optional `defaults:` map of services.yaml
// (type prefix -> Terraform type) and keys it by icon family.
func (c *Catalog) familyDefaults(provider string) map[string]string {
	out := map[string]string{}
	for prefix, tf := range c.serviceDefaults[provider] {
		if icon, ok := c.resourceIcons[provider][prefix]; ok {
			out[icon] = tf
		} else if icon, ok := c.serviceIcons[provider][prefix]; ok {
			out[icon] = tf
		}
	}
	return out
}

// ownerOf returns the type that owns an attachment type, if the schemas say so.
func (c *Catalog) ownerOf(provider, tfType string) string {
	pc := c.Providers[provider]
	types := c.registry.Types(pc.LocalName())
	isType := map[string]bool{}
	for _, t := range types {
		isType[t] = true
	}
	if ext := extendedType(tfType, isType); ext != "" {
		return ext
	}
	props, required, _, ok := c.registry.Resource(tfType)
	if !ok {
		return ""
	}
	for _, owner := range types {
		if owner == tfType || service(owner) != service(tfType) || c.isAttachment(provider, owner) {
			continue
		}
		related := c.iconKey(provider, owner) == c.iconKey(provider, tfType)
		if bindsTo(tfType, owner, props, required, related) != "" {
			return owner
		}
	}
	return ""
}

// familyLabel turns "aws/services/amazon_simple_notification_service.svg" into
// "Simple Notification Service".
func familyLabel(icon string) string {
	name := strings.TrimSuffix(path.Base(icon), ".svg")
	for _, p := range []string{"amazon_", "aws_", "azure_", "microsoft_", "google_"} {
		name = strings.TrimPrefix(name, p)
	}
	words := strings.Split(name, "_")
	for i, w := range words {
		if up, ok := acronyms[w]; ok {
			words[i] = up
		} else if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
