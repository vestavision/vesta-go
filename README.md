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
done, err := client.Wait(ctx, req.RequestID, 2*time.Second, 2*time.Minute)
```

Default base URL: `https://api.vestavision.io`. Override with `vesta.WithBaseURL(...)`.

Auth header: `Authorization: Key vh_…`

OpenAPI: see Vestavision Hub docs (`model-api.v1.yaml`).
