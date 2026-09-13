# Icon attribution

The `.svg` files in this directory and its `services/`, `resources/` and `groups/`
subdirectories (except `generic.svg` and `security_group.svg`) are from the
**AWS Architecture Icons** package, release 07312026, published by Amazon Web
Services at https://aws.amazon.com/architecture/icons/ and used here to depict
AWS services in architecture diagrams, as that page permits. AWS, Amazon Web
Services and the service names and icons are trademarks of Amazon.com, Inc. or
its affiliates. Their inclusion does not imply endorsement by AWS.

`security_group.svg` is drawn by the iagram project (Apache-2.0).

Layout: `services/` = Architecture-Service-Icons (`Arch_*_48.svg`, one per
service, palette tiles), `resources/` = Resource-Icons (`Res_*_48.svg` light
theme, line-art sub-resources such as NAT gateway or S3 bucket), `groups/` =
Architecture-Group-Icons (`*_32.svg`, the small icon in a container's header).
File names are the package names lower-cased with `_` separators.

To refresh: download the current package from the page above, extract the
three sets above with the same normalisation, and update `services.yaml`.
The provider-neutral general icons (user, client, internet, ...) live in
`catalog/icons/common/` and come from the same package (`Res_General-Icons`).
