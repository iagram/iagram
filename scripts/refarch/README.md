# Reference architecture sources

`aws.yaml`, `gcp.yaml` and `azure.yaml` are the research catalogues of
official reference architectures (title, source page, containers, elements as
Terraform types, actors, flows). `convert.py` turns them into
`templates/<provider>/<slug>/template.yaml` specs, asking a running
`iagram up` server which elements exist, which are configuration (dropped)
and which links join two elements:

```
iagram up --port 7780 --no-open &      # in any folder with an iagram.iad
python3 scripts/refarch/convert.py aws scripts/refarch/aws.yaml templates http://127.0.0.1:7780
go run ./cmd/iagram templates build /tmp/out   # renders and validates every template
```

The landing-zone templates are hand-written. Templates are the source of
truth once generated; edit them freely.
