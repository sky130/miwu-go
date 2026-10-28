package home

type ListHomesOption func(*listHomesParams)

type listHomesParams struct {
	fetchShare    bool
	fetchShareDev bool
	appVer        int
	limit         int
}

func defaultListHomesParams() listHomesParams {
	return listHomesParams{fetchShare: true, fetchShareDev: true, appVer: 7, limit: 300}
}

func WithFetchShare(b bool) ListHomesOption {
	return func(p *listHomesParams) { p.fetchShare = b }
}

func WithFetchShareDev(b bool) ListHomesOption {
	return func(p *listHomesParams) { p.fetchShareDev = b }
}

func WithAppVer(v int) ListHomesOption {
	return func(p *listHomesParams) { p.appVer = v }
}

func WithHomeLimit(n int) ListHomesOption {
	return func(p *listHomesParams) { p.limit = n }
}

type ListDevicesOption func(*listDevicesParams)

type listDevicesParams struct {
	limit int
}

func defaultListDevicesParams() listDevicesParams {
	return listDevicesParams{limit: 200}
}

func WithDeviceLimit(n int) ListDevicesOption {
	return func(p *listDevicesParams) { p.limit = n }
}
