# vesta-go

Official Go SDK for the [Vestavision Model API](https://api.vestavision.io).

```bash
go get github.com/vestavision/vesta-go@latest
```

```go
client := vesta.NewClient(os.Getenv("VESTA_API_KEY"))

models, err := client.ListModels(ctx)

req, err := client.Submit(ctx, "vesta/face-landmarks", vesta.SubmitOptions{
    Input: map[string]any{"image_url": "https://example.com/face.jpg"},
})

// One-shot job snapshot (status / output / error)
job, err := client.View(ctx, req.RequestID)

// Block until completed | failed | canceled
done, err := client.Wait(ctx, req.RequestID, 2*time.Second, 2*time.Minute)

// Recent jobs for the organization
jobs, err := client.ListRequests(ctx)
```

| Method | Use |
|--------|-----|
| `Submit` | Start inference |
| `View` / `GetRequest` | Read job once |
| `Wait` | Poll until terminal |
| `ListRequests` | List recent jobs |
| `Cancel` | Cancel a job |
| `Credits` | Credit balance |
| `PresignUpload` | Upload URL for inputs |

Default base URL: `https://api.vestavision.io`. Override with `vesta.WithBaseURL(...)`.

Auth header: `Authorization: Key vh_…`

OpenAPI: Vestavision Hub docs (`model-api.v1.yaml`).
