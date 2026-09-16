# Reference architecture templates

`templates/<provider>/<slug>/template.yaml` describes one reference
architecture. The Go package `internal/templates` renders it into an `.iad`
document at load time (layout included), the gallery lists it, and
`iagram templates build DIR` writes the rendered files. A test renders,
validates and generates every shipped template.

```yaml
title: Transit Gateway hub and spoke
category: networking      # networking, web, serverless, containers, data, ml, resilience, security, iot, devops
tags: [transit gateway, hub and spoke]
source: https://docs.aws.amazon.com/...        # the official page the diagram comes from
description: Two sentences shown on the card.
nodes:
  - {id: users, type: common.users, name: users, step: "1"}
  - {id: acct, type: account, name: prod}                   # short type = <provider>.<type>
  - {id: reg, type: region, name: eu-west-1, parent: acct, props: {region: eu-west-1}}
  - {id: tgw, tf: aws_ec2_transit_gateway, name: hub, parent: reg}   # tf = Terraform type -> curated element importing it, else generated element
edges:
  - {from: users, to: tgw, label: HTTPS, step: "1"}          # kind from the catalog rule (curated, link, references, or flow)
  - {from: tgw, to: vpc-a, link: aws_ec2_transit_gateway_vpc_attachment}   # a link resource drawn as a line
steps:
  - {n: "1", text: "What happens at step 1."}
```

Rules:

- Parents must be declared before their children. Ids default to a slug of
  the name.
- Property defaults come from the catalog; required properties nobody
  supplies get a placeholder (`changeme`, `my-project-123456`, the all-zero
  subscription id, `{}` for blocks) so the template validates and shows the
  user what to fill in.
- Containers that `provide` values (resource groups, projects, zones) supply
  them to the generated resources inside; generated resources take the
  element's name as their `name` attribute.
- An edge with no matching rule is a documentation arrow (`flow`).

### Arrangement from the reference (`at`)

A node may carry `at: [row, col]` or `at: [row, col, rowspan, colspan]`
(1-based): the grid cell it occupies **inside its parent**, read off the
reference diagram (rows top to bottom, columns left to right). The layout
places a parent's children on that grid: columns are as wide as their widest
cell, rows as tall as their tallest, containers stretch to fill their cell
(uniform Availability Zone boxes), leaves sit centred, a spanning box takes
the cells it covers, and children without a cell go in an extra row under the
grid. Cells only order siblings; sizes still come from the content. Without
`at` the flow heuristic arranges the children. Cells are kept in the `.iad`
(`layout.row`, `layout.col`, `layout.row_span`, `layout.col_span`) and
survive Arrange > Horizontal flow; other algorithms ignore them.

```yaml
- {id: app-zone, type: route53_zone, name: app-zone, parent: reg, at: [1, 1]}
- {id: vpc-web, type: vpc, name: web, parent: reg, at: [1, 2, 3, 1]}   # beside three rows of edge services
- {id: az-a, type: availability_zone, name: a, parent: vpc-web, at: [1, 2]}
- {id: sub-public-a, type: subnet, name: public-a, parent: az-a, at: [1, 1]}
```
