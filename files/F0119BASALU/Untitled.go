// Service is asset service.
type Service struct {
	assetRepo   AssetRepository
	deviceRepo  DeviceRepository
	traitRepo   TraitRepository
}

// Materialize is a factory for Service.
func Materialize(mx *materialize.Context) (*Service, error) {
	srv := &Service{}
	mx.Resolve(srv).
		Materialize(&srv.assetRepo, "asset").
		Materialize(&srv.deviceRepo, "device").
		Materialize(&srv.traitRepo, "trait")
	if err := mx.Error(); err != nil {
		return nil, err
	}
	return srv, nil
}