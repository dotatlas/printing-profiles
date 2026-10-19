# Printing Profiles

A community website for sharing 3D printing slicer presets: settings for a specific material on a specific printer, backed by photos of real prints, with reviews and comments.

## Run the API locally

Needs Go 1.27 or newer. Dev needs no settings; `.env.example` lists the variables.

```bash
go -C api run . serve      # then open http://localhost:8080/api/v1/health
go -C api test ./...
```

## License

- **Code:** [GNU AGPL-3.0](LICENSE). If you run a modified copy as a public website, you must publish your changes.
- **Uploaded presets:** CC BY 4.0 by default (share and adapt with credit). Uploaders can choose another license for each preset.
