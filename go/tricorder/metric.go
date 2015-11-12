package tricorder

import (
	"github.com/Symantec/tricorder/go/tricorder/messages"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	panicBadFunctionReturnTypes = "Functions must return either T or (T, error) where T is a primitive numeric type or a string."
	panicInvalidMetric          = "Invalid metric type."
	panicIncompatibleTypes      = "Wrong AsXXX function called on value."
)

var (
	root = newDirectory()
)

// session represents one request to retrieve various metrics.
// session instances handle the calling of region update functions.
// Callers may pass nil for *session parameter in which case
// it is up to the function to create its own session if necessary.
type session struct {
	visitedRegions map[*region]bool
}

func newSession() *session {
	return &session{visitedRegions: make(map[*region]bool)}
}

// Visit indicates that caller is about to fetch metrics from a
// particular region. If this is the session's first time to visit the
// region and no other session is currently visiting it, then
// Visit calls the region's update function.
func (s *session) Visit(r *region) {
	if !s.visitedRegions[r] {
		r.RLock()
		s.visitedRegions[r] = true
	}
}

// Close signals that the caller has retrieved all metrics for the
// particular request. In particular Close indicates that this session
// is finished visiting its regions.
func (s *session) Close() error {
	for visitedRegion := range s.visitedRegions {
		visitedRegion.RUnlock()
	}
	return nil
}

type region struct {
	sendCh     chan int
	receiveCh  chan bool
	lockCount  int
	updateFunc func()
}

func newRegion(updateFunc func()) *region {
	result := &region{
		sendCh:     make(chan int),
		receiveCh:  make(chan bool),
		updateFunc: updateFunc}
	go func() {
		result.handleRequests()
	}()
	return result
}

func (r *region) handleRequests() {
	for {
		prevLockCount := r.lockCount
		r.lockCount += <-r.sendCh
		if r.lockCount < 0 {
			panic("Lock count fell below 0")
		}
		// Update region for first lock holders
		if prevLockCount == 0 && r.lockCount > 0 {
			r.updateFunc()
		}
		r.receiveCh <- true
	}
}

func (r *region) RLock() {
	r.sendCh <- 1
	<-r.receiveCh
}

func (r *region) RUnlock() {
	r.sendCh <- -1
	<-r.receiveCh
}

// bucketPiece represents a single range in a distribution
type bucketPiece struct {
	// Start value of range inclusive
	Start float64
	// End value of range exclusive
	End float64
	// If true range is of the form < End.
	First bool
	// If true range is of the form >= Start.
	Last bool
}

func newExponentialBucketerStream(
	count int, start, scale float64) []float64 {
	if count < 2 || start <= 0.0 || scale <= 1.0 {
		panic("count >= 2 && start > 0.0 && scale > 1")
	}
	result := make([]float64, count-1)
	current := start
	for i := 0; i < count-1; i++ {
		result[i] = current
		current *= scale
	}
	return result
}

func newLinearBucketerStream(
	count int, start, increment float64) []float64 {
	if count < 2 || increment <= 0.0 {
		panic("count >= 2 && increment > 0")
	}
	result := make([]float64, count-1)
	current := start
	for i := 0; i < count-1; i++ {
		result[i] = current
		current += increment
	}
	return result
}

var (
	coefficientsForGeometricBucketer = []float64{1.0, 2.0, 5.0}
)

func findGeometricBaseAndOffset(x float64) (base float64, offset int) {
	base = 1.0
	for x >= 10.0 {
		x /= 10.0
		base *= 10.0
	}
	for x < 1.0 {
		x *= 10.0
		base /= 10.0
	}
	coeffLen := len(coefficientsForGeometricBucketer)
	for i := coeffLen - 1; i >= 0; i-- {
		if x >= coefficientsForGeometricBucketer[i] {
			return base, i
		}
	}
	return base / 10.0, coeffLen - 1
}

func newGeometricBucketerStream(lower, upper float64) (result []float64) {
	if lower <= 0 || upper < lower {
		panic("lower must > 0 and upper >= lower")
	}
	base, offset := findGeometricBaseAndOffset(lower)
	coeffLen := len(coefficientsForGeometricBucketer)
	emitValue := base * coefficientsForGeometricBucketer[offset]
	for emitValue < upper {
		result = append(result, emitValue)
		offset++
		if offset == coeffLen {
			base *= 10
			offset = 0
		}
		emitValue = base * coefficientsForGeometricBucketer[offset]
	}
	result = append(result, emitValue)
	return
}

func newBucketerFromEndpoints(endpoints []float64) *Bucketer {
	var pieces []*bucketPiece
	lower := endpoints[0]
	pieces = append(pieces, &bucketPiece{First: true, End: lower})
	for _, upper := range endpoints[1:] {
		pieces = append(
			pieces, &bucketPiece{Start: lower, End: upper})
		lower = upper
	}
	pieces = append(
		pieces, &bucketPiece{Last: true, Start: lower})
	return &Bucketer{pieces: pieces}
}

// breakdownPiece represents a single range and count pair in a
// distribution breakdown
type breakdownPiece struct {
	*bucketPiece
	Count uint64
}

// breakdown represents a distribution breakdown.
type breakdown []breakdownPiece

// snapshot represents a snapshot of a distribution
type snapshot struct {
	Min     float64
	Max     float64
	Average float64

	// TODO: Have to discuss how to implement this
	Median    float64
	Count     uint64
	Breakdown breakdown
}

// distribution represents a distribution of values same as Distribution
type distribution struct {
	// Protects all fields except pieces whose contents never changes
	lock   sync.RWMutex
	pieces []*bucketPiece
	counts []uint64
	total  float64
	min    float64
	max    float64
	count  uint64
}

func newDistribution(bucketer *Bucketer) *distribution {
	return &distribution{
		pieces: bucketer.pieces,
		counts: make([]uint64, len(bucketer.pieces)),
	}
}

// Add adds a value to this distribution
func (d *distribution) Add(value float64) {
	idx := findDistributionIndex(d.pieces, value)
	d.lock.Lock()
	defer d.lock.Unlock()
	d.counts[idx]++
	d.total += value
	if d.count == 0 {
		d.min = value
		d.max = value
	} else if value < d.min {
		d.min = value
	} else if value > d.max {
		d.max = value
	}
	d.count++
}

func findDistributionIndex(pieces []*bucketPiece, value float64) int {
	return sort.Search(len(pieces)-1, func(i int) bool {
		return value < pieces[i].End
	})
}

func valueIndexToPiece(counts []uint64, valueIdx float64) (
	pieceIdx int, frac float64) {
	pieceIdx = 0
	startValueIdxInPiece := -0.5
	for valueIdx-startValueIdxInPiece >= float64(counts[pieceIdx]) {
		startValueIdxInPiece += float64(counts[pieceIdx])
		pieceIdx++
	}
	return pieceIdx, (valueIdx - startValueIdxInPiece) / float64(counts[pieceIdx])

}

func interpolate(min float64, max float64, frac float64) float64 {
	return (1.0-frac)*min + frac*max
}

func (d *distribution) calculateMedian() float64 {
	if d.count == 1 {
		return d.min
	}
	medianIndex := float64(d.count-1) / 2.0
	pieceIdx, frac := valueIndexToPiece(d.counts, medianIndex)
	pieceLen := len(d.pieces)
	if pieceIdx == 0 {
		return interpolate(
			d.min, math.Min(d.pieces[0].End, d.max), frac)
	}
	if pieceIdx == pieceLen-1 {
		return interpolate(math.Max(d.pieces[pieceLen-1].Start, d.min), d.max, frac)
	}
	return interpolate(
		math.Max(d.pieces[pieceIdx].Start, d.min),
		math.Min(d.pieces[pieceIdx].End, d.max),
		frac)
}

// Snapshot fetches the snapshot of this distribution atomically
func (d *distribution) Snapshot() *snapshot {
	bdn := make(breakdown, len(d.pieces))
	for i := range bdn {
		bdn[i].bucketPiece = d.pieces[i]
	}
	d.lock.RLock()
	defer d.lock.RUnlock()
	for i := range bdn {
		bdn[i].Count = d.counts[i]
	}
	if d.count == 0 {
		return &snapshot{
			Count:     d.count,
			Breakdown: bdn,
		}
	}
	return &snapshot{
		Min:       d.min,
		Max:       d.max,
		Average:   d.total / float64(d.count),
		Median:    d.calculateMedian(),
		Count:     d.count,
		Breakdown: bdn,
	}

}

// value represents the value of a metric.
type value struct {
	val           reflect.Value
	region        *region
	dist          *distribution
	valType       types.Type
	isValAPointer bool
	isfunc        bool
}

var (
	timePtrType  = reflect.TypeOf((*time.Time)(nil))
	timeType     = timePtrType.Elem()
	durationType = reflect.TypeOf(time.Duration(0))
)

// Given a type t from the reflect package, return the corresponding
// types.Type and true if t is a pointer to that type or false otherwise.
func getPrimitiveType(t reflect.Type) (types.Type, bool) {
	switch t {
	case timePtrType:
		return types.Time, true
	case timeType:
		return types.Time, false
	case durationType:
		return types.Duration, false
	default:
		switch t.Kind() {
		case reflect.Bool:
			return types.Bool, false
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return types.Int, false
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return types.Uint, false
		case reflect.Float32, reflect.Float64:
			return types.Float, false
		case reflect.String:
			return types.String, false
		default:
			panic(panicInvalidMetric)
		}
	}
}

func newValue(spec interface{}, region *region) *value {
	capDist, ok := spec.(*Distribution)
	if ok {
		return &value{dist: (*distribution)(capDist), valType: types.Dist}
	}
	dist, ok := spec.(*distribution)
	if ok {
		return &value{dist: dist, valType: types.Dist}
	}
	v := reflect.ValueOf(spec)
	t := v.Type()
	if t.Kind() == reflect.Func {
		funcArgCount := t.NumOut()

		// Our functions have to return exactly one thing
		if funcArgCount != 1 {
			panic(panicBadFunctionReturnTypes)
		}
		valType, isValAPointer := getPrimitiveType(t.Out(0))
		return &value{
			val:           v,
			valType:       valType,
			isfunc:        true,
			isValAPointer: isValAPointer}
	}
	v = v.Elem()
	valType, isValAPointer := getPrimitiveType(v.Type())
	return &value{
		val:           v,
		region:        region,
		valType:       valType,
		isValAPointer: isValAPointer}
}

// Type returns the type of this value: Int, Float, Uint, String, or Dist
func (v *value) Type() types.Type {
	return v.valType
}

func (v *value) evaluate(s *session) reflect.Value {
	if !v.isfunc {
		if v.region != nil {
			if s == nil {
				s = newSession()
				defer s.Close()
			}
			s.Visit(v.region)
		}
		return v.val
	}
	result := v.val.Call(nil)
	return result[0]
}

// AsXXX methods return this value as a type XX.
// AsXXX methods panic if this value is not of type XX.
// If the caller passes a nil session to an AsXXX method,
// it creates its own session internally.
func (v *value) AsBool(s *session) bool {
	if v.valType != types.Bool {
		panic(panicIncompatibleTypes)
	}
	return v.evaluate(s).Bool()
}

func (v *value) AsInt(s *session) int64 {
	if v.valType != types.Int && v.valType != types.Duration {
		panic(panicIncompatibleTypes)
	}
	return v.evaluate(s).Int()
}

func (v *value) AsUint(s *session) uint64 {
	if v.valType != types.Uint {
		panic(panicIncompatibleTypes)
	}
	return v.evaluate(s).Uint()
}

func (v *value) AsFloat(s *session) float64 {
	if v.valType != types.Float {
		panic(panicIncompatibleTypes)
	}
	return v.evaluate(s).Float()
}

func (v *value) AsString(s *session) string {
	if v.valType != types.String {
		panic(panicIncompatibleTypes)
	}
	return v.evaluate(s).String()
}

func (v *value) AsTime(s *session) (result time.Time) {
	if v.valType != types.Time {
		panic(panicIncompatibleTypes)
	}
	val := v.evaluate(s)
	if v.isValAPointer {
		p := val.Interface().(*time.Time)
		if p == nil {
			return
		}
		return *p
	}
	return val.Interface().(time.Time)
}

func (v *value) AsGoDuration(s *session) time.Duration {
	return time.Duration(v.AsInt(s))
}

func (v *value) AsDuration(s *session) (result messages.Duration) {
	if v.valType == types.Time {
		t := v.AsTime(s)
		if t.IsZero() {
			return
		}
		return messages.SinceEpoch(t)
	}
	if v.valType == types.Duration {
		return messages.NewDuration(v.AsGoDuration(s))
	}
	panic(panicIncompatibleTypes)
}

func asJSONRanges(ranges breakdown) []*messages.RangeWithCount {
	result := make([]*messages.RangeWithCount, len(ranges))
	for i := range ranges {
		result[i] = &messages.RangeWithCount{Count: ranges[i].Count}
		if !ranges[i].First {
			result[i].Lower = &ranges[i].Start
		}
		if !ranges[i].Last {
			result[i].Upper = &ranges[i].End
		}
	}
	return result
}

func asRPCRanges(ranges breakdown) []*messages.RpcRangeWithCount {
	result := make([]*messages.RpcRangeWithCount, len(ranges))
	for i := range ranges {
		result[i] = &messages.RpcRangeWithCount{
			Count: ranges[i].Count,
			Lower: ranges[i].Start,
			Upper: ranges[i].End}
	}
	return result
}

// AsJsonValue returns this value as a messages.Value.
func (v *value) AsJsonValue(s *session) *messages.Value {
	t := v.Type()
	switch t {
	case types.Bool:
		b := v.AsBool(s)
		return &messages.Value{Kind: t, BoolValue: &b}
	case types.Int:
		i := v.AsInt(s)
		return &messages.Value{Kind: t, IntValue: &i}
	case types.Uint:
		u := v.AsUint(s)
		return &messages.Value{Kind: t, UintValue: &u}
	case types.Float:
		f := v.AsFloat(s)
		return &messages.Value{Kind: t, FloatValue: &f}
	case types.String:
		s := v.AsString(s)
		return &messages.Value{Kind: t, StringValue: &s}
	case types.Time, types.Duration:
		s := v.AsTextString(s)
		return &messages.Value{Kind: t, StringValue: &s}
	case types.Dist:
		snapshot := v.AsDistribution().Snapshot()
		return &messages.Value{
			Kind: t,
			DistributionValue: &messages.Distribution{
				Min:     snapshot.Min,
				Max:     snapshot.Max,
				Average: snapshot.Average,
				Median:  snapshot.Median,
				Count:   snapshot.Count,
				Ranges:  asJSONRanges(snapshot.Breakdown)}}
	default:
		panic(panicIncompatibleTypes)
	}
}

// AsRpcValue returns this value as a messages.RpcValue.
func (v *value) AsRpcValue(s *session) *messages.RpcValue {
	t := v.Type()
	switch t {
	case types.Bool:
		return &messages.RpcValue{Kind: t, BoolValue: v.AsBool(s)}
	case types.Int:
		return &messages.RpcValue{Kind: t, IntValue: v.AsInt(s)}
	case types.Uint:
		return &messages.RpcValue{Kind: t, UintValue: v.AsUint(s)}
	case types.Float:
		return &messages.RpcValue{Kind: t, FloatValue: v.AsFloat(s)}
	case types.String:
		return &messages.RpcValue{Kind: t, StringValue: v.AsString(s)}
	case types.Time, types.Duration:
		return &messages.RpcValue{Kind: t, DurationValue: v.AsDuration(s)}
	case types.Dist:
		snapshot := v.AsDistribution().Snapshot()
		return &messages.RpcValue{
			Kind: t,
			DistributionValue: &messages.RpcDistribution{
				Min:     snapshot.Min,
				Max:     snapshot.Max,
				Average: snapshot.Average,
				Median:  snapshot.Median,
				Count:   snapshot.Count,
				Ranges:  asRPCRanges(snapshot.Breakdown)}}
	default:
		panic(panicIncompatibleTypes)
	}
}

// AsTextString returns this value as a text friendly string.
// AsTextString panics if this value does not represent a single value.
// For example, AsTextString panics if this value represents a distribution.
// If caller passes a nil session, AsTextString creates its own internally.
func (v *value) AsTextString(s *session) string {
	switch v.Type() {
	case types.Bool:
		if v.AsBool(s) {
			return "true"
		}
		return "false"
	case types.Int:
		return strconv.FormatInt(v.AsInt(s), 10)
	case types.Uint:
		return strconv.FormatUint(v.AsUint(s), 10)
	case types.Float:
		return strconv.FormatFloat(v.AsFloat(s), 'f', -1, 64)
	case types.String:
		return "\"" + v.AsString(s) + "\""
	case types.Time, types.Duration:
		return v.AsDuration(s).String()
	default:
		panic(panicIncompatibleTypes)
	}
}

// AsHtmlString returns this value as an html friendly string.
// AsHtmlString panics if this value does not represent a single value.
// For example, AsHtmlString panics if this value represents a distribution.
// If caller passes a nil session, AsHtmlString creates its own internally.
func (v *value) AsHtmlString(s *session) string {
	switch v.Type() {
	//TODO: ISO-8601 for duration?
	// The most we can include is hours because of daylight
	// savings. May not be worh it?
	// github.com/ChannelMeter/iso8601duration can do ISO-8601 for us.
	case types.Time:
		t := v.AsTime(s).UTC()
		return t.Format("2006-01-02T15:04:05.999999999Z")
	default:
		return v.AsTextString(s)
	}
}

// AsDistribution returns this value as a Distribution.
// AsDistribution panics if this value does not represent a distribution
func (v *value) AsDistribution() *distribution {
	if v.valType != types.Dist {
		panic(panicIncompatibleTypes)
	}
	return v.dist
}

// metric represents a single metric.
type metric struct {
	// The description of the metric
	Description string
	// The unit of measurement
	Unit units.Unit
	// The value of the metric
	*value
	enclosingListEntry *listEntry
}

// AbsPath returns the absolute path of this metric
func (m *metric) AbsPath() string {
	return m.enclosingListEntry.absPath()
}

// listEntry represents a single entry in a directory listing.
type listEntry struct {
	// The name of this list entry.
	Name string
	// If this list entry represents a metric, Metric is non-nil
	Metric *metric
	// If this list entry represents a directory, Directory is non-nil
	Directory *directory
	parent    *listEntry
}

func (n *listEntry) pathFrom(fromDir *directory) pathSpec {
	var names pathSpec
	current := n
	from := fromDir.enclosingListEntry
	for ; current != nil && current != from; current = current.parent {
		names = append(names, current.Name)
	}
	if current != from {
		return nil
	}
	pathLen := len(names)
	for i := 0; i < pathLen/2; i++ {
		names[i], names[pathLen-i-1] = names[pathLen-i-1], names[i]
	}
	return names
}

func (n *listEntry) absPath() string {
	return "/" + n.pathFrom(root).String()
}

// metricsCollector represents any data structure used to collect metrics.
type metricsCollector interface {
	// Collect collects a single metric. Implementations may assume
	// that s is non nil.
	Collect(m *metric, s *session) error
}

// directory represents a directory same as DirectorySpec
type directory struct {
	contents           map[string]*listEntry
	enclosingListEntry *listEntry
}

func newDirectory() *directory {
	return &directory{contents: make(map[string]*listEntry)}
}

// List lists the contents of this directory in lexographical order by name.
func (d *directory) List() []*listEntry {
	result := make([]*listEntry, len(d.contents))
	idx := 0
	for _, n := range d.contents {
		result[idx] = n
		idx++
	}
	return sortListEntries(result)
}

// AbsPath returns the absolute path of this directory
func (d *directory) AbsPath() string {
	return d.enclosingListEntry.absPath()
}

// GetDirectory returns the directory with the given relative
// path or nil if no such directory exists.
func (d *directory) GetDirectory(relativePath string) *directory {
	return d.getDirectory(newPathSpec(relativePath))
}

// GetMetric returns the metric with the given relative
// path or nil if no such metric exists.
func (d *directory) GetMetric(relativePath string) *metric {
	_, m := d.getDirectoryOrMetric(newPathSpec(relativePath))
	return m
}

// GetAllMetricsByPath does a depth first traversal from relativePath to
// find all the metrics and store them within collector. If relativePath
// denotes a single metric, then GetAllMetricsByPath stores that single metric
// within collector. If relativePath does not exist in this directory, then
// GetAllMetricsByPath stores nothing in collector.
// If the Collect() method of collector returns a non nil error,
// GetAllMetricsByPath stops traversal and returns that same error.
// Callers may pass nil for the session in which case
// GetAllMetricsByPath() creates its own session object internally.
// However, if a caller is calling GetAllMetricsByPath() multiple times
// to service the same request, the caller should create a session
// for the request, pass it whenever a session is required and finally
// close the session when done servicing the request.
func (d *directory) GetAllMetricsByPath(
	relativePath string, collector metricsCollector, s *session) error {
	dir, m := d.GetDirectoryOrMetric(relativePath)
	if m != nil {
		if s == nil {
			s = newSession()
			defer s.Close()
		}
		return collector.Collect(m, s)
	} else if dir != nil {
		return dir.GetAllMetrics(collector, s)
	}
	return nil
}

// GetDirectoryOrMetric returns either the directory or metric
// at the given path while traversing the directory tree just one time.
// If path not found: returns nil, nil; if path is a directory:
// returns d, nil; if path is a metric: returns nil, m.
func (d *directory) GetDirectoryOrMetric(relativePath string) (
	*directory, *metric) {
	return d.getDirectoryOrMetric(newPathSpec(relativePath))
}

// GetAllMetricsByPath does a depth first traversal of this directory to
// find all the metrics and store them within collector.
// If the Collect() method of collector returns a non nil error,
// GetAllMetrics stops traversal and returns that same error.
// Callers may pass nil for the session in which case
// GetAllMetric() creates its own session object internally.
// However, if a caller is calling GetAllMetrics() multiple times
// to service the same request, the caller should create a session
// for the request, pass it whenever a session is required and
// finally close the session when done servicing the request.
func (d *directory) GetAllMetrics(
	collector metricsCollector, s *session) (err error) {
	if s == nil {
		s = newSession()
		defer s.Close()
	}
	for _, entry := range d.List() {
		if entry.Directory != nil {
			err = entry.Directory.GetAllMetrics(collector, s)
		} else {
			err = collector.Collect(entry.Metric, s)
		}
		if err != nil {
			return
		}
	}
	return
}

func (d *directory) getDirectory(path pathSpec) (result *directory) {
	result = d
	for _, part := range path {
		n := result.contents[part]
		if n == nil || n.Directory == nil {
			return nil
		}
		result = n.Directory
	}
	return
}

func (d *directory) getDirectoryOrMetric(path pathSpec) (
	*directory, *metric) {
	if path.Empty() {
		return d, nil
	}
	dir := d.getDirectory(path.Dir())
	if dir == nil {
		return nil, nil
	}
	n := dir.contents[path.Base()]
	if n == nil {
		return nil, nil
	}
	return n.Directory, n.Metric
}

func (d *directory) createDirIfNeeded(name string) (*directory, error) {
	n := d.contents[name]

	// We need to create the new directory
	if n == nil {
		newDir := newDirectory()
		newListEntry := &listEntry{
			Name: name, Directory: newDir, parent: d.enclosingListEntry}
		newDir.enclosingListEntry = newListEntry
		d.contents[name] = newListEntry
		return newDir, nil
	}

	// The directory already exists
	if n.Directory != nil {
		return n.Directory, nil
	}

	// name already associated with a metric, return error
	return nil, ErrPathInUse
}

func (d *directory) storeMetric(name string, m *metric) error {
	n := d.contents[name]
	// Oops something already stored under name, return error
	if n != nil {
		return ErrPathInUse
	}
	newListEntry := &listEntry{Name: name, Metric: m, parent: d.enclosingListEntry}
	m.enclosingListEntry = newListEntry
	d.contents[name] = newListEntry
	return nil
}

func (d *directory) registerDirectory(path pathSpec) (
	result *directory, err error) {
	result = d
	for _, part := range path {
		result, err = result.createDirIfNeeded(part)
		if err != nil {
			return
		}
	}
	return
}

func (d *directory) registerMetric(
	path pathSpec,
	value interface{},
	region *region,
	unit units.Unit,
	description string) (err error) {
	if path.Empty() {
		return ErrPathInUse
	}
	current, err := d.registerDirectory(path.Dir())
	if err != nil {
		return
	}
	metric := &metric{
		Description: description,
		Unit:        unit,
		value:       newValue(value, region)}
	return current.storeMetric(path.Base(), metric)
}

// pathSpec represents a relative path
type pathSpec []string

func newPathSpec(path string) pathSpec {
	parts := strings.Split(path, "/")

	// Filter out empty path parts
	idx := 0
	for i := range parts {
		if strings.TrimSpace(parts[i]) == "" {
			continue
		}
		parts[idx] = parts[i]
		idx++
	}
	return parts[:idx]
}

// Dir returns the directory part of the path
// Dir panics if this path is empty
func (p pathSpec) Dir() pathSpec {
	plen := len(p)
	if plen == 0 {
		panic("Can't take Dir() of empty path")
	}
	return p[:plen-1]
}

// Base returns the name part of the path
// Base panics if this path is empty
func (p pathSpec) Base() string {
	plen := len(p)
	if plen == 0 {
		panic("Can't take Base() of empty path")
	}
	return p[plen-1]
}

// Empty returns true if this path is empty
func (p pathSpec) Empty() bool {
	return len(p) == 0
}

func (p pathSpec) String() string {
	return strings.Join(p, "/")
}

// byName sorts list entries by name
type byName []*listEntry

func (b byName) Len() int {
	return len(b)
}

func (b byName) Less(i, j int) bool {
	return b[i].Name < b[j].Name
}

func (b byName) Swap(i, j int) {
	b[j], b[i] = b[i], b[j]
}

func sortListEntries(listEntries []*listEntry) []*listEntry {
	sort.Sort(byName(listEntries))
	return listEntries
}
