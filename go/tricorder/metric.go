package tricorder

import (
	"flag"
	"fmt"
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
	"unsafe"
)

const (
	panicBadFunctionReturnTypes = "Functions must return either T or (T, error) where T is a primitive numeric type or a string."
	panicInvalidMetric          = "Invalid metric type."
	panicIncompatibleTypes      = "Wrong AsXXX function called on value."
	panicTypeMismatch           = "Wrong type passed to method."
)

var (
	root          = newDirectory()
	intSizeInBits = int(unsafe.Sizeof(0)) * 8
)

type rpcEncoding int

const (
	jsonEncoding rpcEncoding = iota
	goRpcEncoding
)

var (
	suffixes = []string{
		" thousand", " million", " billion", " trillion",
		"K trillion", "M trillion"}
	byteSuffixes = []string{
		" KiB", " MiB", " GiB", " TiB", " PiB", " EiB"}
	bytePerSecondSuffixes = []string{
		" KiB/s", " MiB/s", " GiB/s", " TiB/s", " PiB/s", " EiB/s"}
)

var (
	flagUnits = make(map[string]units.Unit)
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
	Min       float64
	Max       float64
	Average   float64
	Median    float64
	Sum       float64
	Count     uint64
	Breakdown breakdown
}

// distribution represents a distribution of values same as Distribution
type distribution struct {
	// Protects all fields except pieces whose contents never changes
	// and unit which never changes after RegisterMetric sets it.
	lock   sync.RWMutex
	pieces []*bucketPiece
	unit   units.Unit
	counts []uint64
	total  float64
	min    float64
	max    float64
	count  uint64
}

func (d *distribution) Sum() float64 {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.total
}

func newDistribution(bucketer *Bucketer) *distribution {
	return &distribution{
		pieces: bucketer.pieces,
		unit:   units.None,
		counts: make([]uint64, len(bucketer.pieces)),
	}
}

func (d *distribution) Add(value interface{}) {
	d.add(d.valueToFloat(value))
}

func (d *distribution) Update(oldValue, newValue interface{}) {
	d.update(d.valueToFloat(oldValue), d.valueToFloat(newValue))
}

func (d *distribution) add(value float64) {
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

func (d *distribution) update(oldValue, newValue float64) {
	if d.count == 0 {
		panic("Can't call update on an empty distribution.")
	}
	oldIdx := findDistributionIndex(d.pieces, oldValue)
	newIdx := findDistributionIndex(d.pieces, newValue)
	d.lock.Lock()
	defer d.lock.Unlock()
	d.counts[newIdx]++
	d.counts[oldIdx]--
	d.total += (newValue - oldValue)
	if newValue < d.min {
		d.min = newValue
	} else if newValue > d.max {
		d.max = newValue
	}
}

func (d *distribution) valueToFloat(value interface{}) float64 {
	switch v := value.(type) {
	case time.Duration:
		return d.goDurationToFloat(v)
	case float32:
		return float64(v)
	case float64:
		return v
	default:
		panic(panicTypeMismatch)
	}
}

func (d *distribution) goDurationToFloat(dur time.Duration) float64 {
	switch d.unit {
	case units.Second:
		return float64(dur) / float64(time.Second)
	case units.Millisecond:
		return float64(dur) / float64(time.Millisecond)
	default:
		panic("Durations can't be added to non time based metrics.")
	}
}

func findDistributionIndex(pieces []*bucketPiece, value float64) int {
	return sort.Search(len(pieces)-1, func(i int) bool {
		return value < pieces[i].End
	})
}

func (d *distribution) indexToValue(valueIdx uint64) float64 {
	pieceIdx := 0
	var startValueIdxInPiece uint64
	for valueIdx-startValueIdxInPiece >= d.counts[pieceIdx] {
		startValueIdxInPiece += d.counts[pieceIdx]
		pieceIdx++
	}
	frac := float64(valueIdx-startValueIdxInPiece+1) / float64(d.counts[pieceIdx]+1)
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

func interpolate(min float64, max float64, frac float64) float64 {
	return (1.0-frac)*min + frac*max
}

func (d *distribution) calculateMedian() float64 {
	if d.count <= 2 {
		return d.total / float64(d.count)
	}
	if d.count%2 == 0 {
		return (d.indexToValue(d.count/2-1) + d.indexToValue(d.count/2)) / 2.0
	}
	return d.indexToValue(d.count / 2)
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
		Sum:       d.total,
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
	bits          int
	isValAPointer bool
	isfunc        bool
	unit          units.Unit
}

var (
	timePtrType  = reflect.TypeOf((*time.Time)(nil))
	timeType     = timePtrType.Elem()
	durationType = reflect.TypeOf(time.Duration(0))
)

// Given a type t from the reflect package, return the corresponding
// types.Type, size in bits,  and true if t is a pointer to that type
// or false otherwise. Returns bits = 0 if size is unknown.
// If t is not supported, returns bits = -1.
func getPrimitiveType(t reflect.Type) (
	outType types.Type, bits int, isPtr bool) {
	switch t {
	case timePtrType:
		return types.Time, 0, true
	case timeType:
		return types.Time, 0, false
	case durationType:
		return types.Duration, 0, false
	default:
		switch t.Kind() {
		case reflect.Bool:
			return types.Bool, 0, false
		case reflect.Int:
			return types.Int, intSizeInBits, false
		case reflect.Int8:
			return types.Int, 8, false
		case reflect.Int16:
			return types.Int, 16, false
		case reflect.Int32:
			return types.Int, 32, false
		case reflect.Int64:
			return types.Int, 64, false
		case reflect.Uint:
			return types.Uint, intSizeInBits, false
		case reflect.Uint8:
			return types.Uint, 8, false
		case reflect.Uint16:
			return types.Uint, 16, false
		case reflect.Uint32:
			return types.Uint, 32, false
		case reflect.Uint64:
			return types.Uint, 64, false
		case reflect.Float32:
			return types.Float, 32, false
		case reflect.Float64:
			return types.Float, 64, false
		case reflect.String:
			return types.String, 0, false
		default:
			bits = -1
			return
		}
	}
}

// like getPrimitiveType but panics if t is not supported
func mustGetPrimitiveType(t reflect.Type) (
	outType types.Type, bits int, isPtr bool) {
	outType, bits, isPtr = getPrimitiveType(t)
	if bits == -1 {
		panic(panicInvalidMetric)
	}
	return
}

// unit parameter only used if spec is a *Distribution
// In that case, it sets the unit of the *Distribution in place.
func newValue(spec interface{}, region *region, unit units.Unit) *value {
	capDist, ok := spec.(*Distribution)
	if ok {
		dist := (*distribution)(capDist)
		dist.unit = unit
		return &value{dist: dist, unit: unit, valType: types.Dist}
	}
	dist, ok := spec.(*distribution)
	if ok {
		dist.unit = unit
		return &value{dist: dist, unit: unit, valType: types.Dist}
	}
	flagGetter, ok := spec.(flag.Getter)
	if ok {
		t := reflect.ValueOf(flagGetter.Get()).Type()
		valType, bits, isValAPointer := getPrimitiveType(t)
		var valFunc reflect.Value
		if bits == -1 {
			valType = types.String
			bits = 0
			isValAPointer = false
			valFunc = reflect.ValueOf(flagGetter.String)
		} else {
			valFunc = reflect.ValueOf(flagGetter.Get)
		}
		return &value{
			val:           valFunc,
			unit:          unit,
			valType:       valType,
			bits:          bits,
			isfunc:        true,
			isValAPointer: isValAPointer}
	}
	v := reflect.ValueOf(spec)
	t := v.Type()
	if t.Kind() == reflect.Func {
		funcArgCount := t.NumOut()

		// Our functions have to return exactly one thing
		if funcArgCount != 1 {
			panic(panicBadFunctionReturnTypes)
		}
		valType, bits, isValAPointer := mustGetPrimitiveType(t.Out(0))
		return &value{
			val:           v,
			unit:          unit,
			region:        region,
			valType:       valType,
			bits:          bits,
			isfunc:        true,
			isValAPointer: isValAPointer}
	}
	v = v.Elem()
	valType, bits, isValAPointer := mustGetPrimitiveType(v.Type())
	return &value{
		val:           v,
		unit:          unit,
		region:        region,
		valType:       valType,
		bits:          bits,
		isValAPointer: isValAPointer}
}

// Type returns the type of this value: Int, Float, Uint, String, or Dist
func (v *value) Type() types.Type {
	return v.valType
}

// Unit returns the unit of this value.
func (v *value) Unit() units.Unit {
	return v.unit
}

// Bits returns the size in bits if the type is an Int, Uint, for Float.
// Otherwise Bits returns 0.
func (v *value) Bits() int {
	return v.bits
}

func (v *value) evaluate(s *session) reflect.Value {
	if v.region != nil {
		if s == nil {
			s = newSession()
			defer s.Close()
		}
		s.Visit(v.region)
	}
	if !v.isfunc {
		return v.val
	}
	result := v.val.Call(nil)[0]
	// Needed as the Get() method on flag values returns an interface{}.
	if result.Type().Kind() == reflect.Interface {
		return result.Elem()
	}
	return result
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
	if v.valType != types.Int {
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
	return time.Duration(v.evaluate(s).Int())
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

func asRanges(ranges breakdown) []*messages.RangeWithCount {
	result := make([]*messages.RangeWithCount, len(ranges))
	for i := range ranges {
		result[i] = &messages.RangeWithCount{
			Count: ranges[i].Count,
			Lower: ranges[i].Start,
			Upper: ranges[i].End}
	}
	return result
}

func (v *value) updateJsonOrRpcMetric(
	s *session, metric *messages.Metric, encoding rpcEncoding) {
	t := v.Type()
	metric.Bits = v.Bits()
	switch t {
	case types.Bool:
		metric.Kind = t
		metric.Value = v.AsBool(s)
	case types.Int:
		metric.Kind = t
		metric.Value = v.AsInt(s)
	case types.Uint:
		metric.Kind = t
		metric.Value = v.AsUint(s)
	case types.Float:
		metric.Kind = t
		metric.Value = v.AsFloat(s)
	case types.String:
		metric.Kind = t
		metric.Value = v.AsString(s)
	case types.Time:
		metric.Kind = types.GoTime
		metric.Value = v.AsTime(s)
	case types.Duration:
		metric.Kind = types.GoDuration
		metric.Value = v.AsGoDuration(s)
	case types.Dist:
		snapshot := v.AsDistribution().Snapshot()
		metric.Kind = t
		metric.Value = &messages.Distribution{
			Min:     snapshot.Min,
			Max:     snapshot.Max,
			Average: snapshot.Average,
			Median:  snapshot.Median,
			Sum:     snapshot.Sum,
			Count:   snapshot.Count,
			Ranges:  asRanges(snapshot.Breakdown)}
	default:
		panic(panicIncompatibleTypes)
	}
	if encoding == jsonEncoding {
		metric.ConvertToJson()
	}
}

// UpdateJsonMetric updates fields in metric for JSON.
func (v *value) UpdateJsonMetric(s *session, metric *messages.Metric) {
	v.updateJsonOrRpcMetric(s, metric, jsonEncoding)
}

// UpdateRpcMetric updates fields in this metric for go RPC.
func (v *value) UpdateRpcMetric(s *session, metric *messages.Metric) {
	v.updateJsonOrRpcMetric(s, metric, goRpcEncoding)
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
		return v.AsDuration(s).StringUsingUnits(v.unit)
	default:
		panic(panicIncompatibleTypes)
	}
}

func iCompactForm(x int64, radix uint, suffixes []string) string {
	if x > -1*int64(radix) && x < int64(radix) {
		return strconv.FormatInt(x, 10)
	}
	idx := -1
	fx := float64(x)
	fradix := float64(radix)
	for fx >= fradix || fx <= -fradix {
		fx /= fradix
		idx++
	}
	switch {
	case fx > -9.995 && fx < 9.995:
		return fmt.Sprintf("%.2f%s", fx, suffixes[idx])
	case fx > -99.95 && fx < 99.95:
		return fmt.Sprintf("%.1f%s", fx, suffixes[idx])
	default:
		return fmt.Sprintf("%.0f%s", fx, suffixes[idx])
	}
}

func uCompactForm(x uint64, radix uint, suffixes []string) string {
	if x < uint64(radix) {
		return strconv.FormatUint(x, 10)
	}
	idx := -1
	fx := float64(x)
	fradix := float64(radix)
	for fx >= fradix {
		fx /= fradix
		idx++
	}
	switch {
	case fx < 9.995:
		return fmt.Sprintf("%.2f%s", fx, suffixes[idx])
	case fx < 99.95:
		return fmt.Sprintf("%.1f%s", fx, suffixes[idx])
	default:
		return fmt.Sprintf("%.0f%s", fx, suffixes[idx])
	}
}

// AsHtmlString returns this value as an html friendly string.
// AsHtmlString panics if this value does not represent a single value.
// For example, AsHtmlString panics if this value represents a distribution.
// If caller passes a nil session, AsHtmlString creates its own internally.
func (v *value) AsHtmlString(s *session) string {
	switch v.Type() {
	case types.Int:
		switch v.unit {
		case units.Byte:
			return iCompactForm(v.AsInt(s), 1024, byteSuffixes)
		case units.BytePerSecond:
			return iCompactForm(
				v.AsInt(s), 1024, bytePerSecondSuffixes)
		default:
			return iCompactForm(v.AsInt(s), 1000, suffixes)
		}
	case types.Uint:
		switch v.unit {
		case units.Byte:
			return uCompactForm(v.AsUint(s), 1024, byteSuffixes)
		case units.BytePerSecond:
			return uCompactForm(
				v.AsUint(s), 1024, bytePerSecondSuffixes)
		default:
			return uCompactForm(v.AsUint(s), 1000, suffixes)
		}
	case types.Duration:
		if s == nil {
			s = newSession()
			defer s.Close()
		}
		d := v.AsDuration(s)
		if d.IsNegative() {
			return v.AsTextString(s)
		}
		return d.PrettyFormat()
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
		value:       newValue(value, region, unit)}
	return current.storeMetric(path.Base(), metric)
}

func registerFlags() {
	flagDirectory, err := RegisterDirectory("/proc/flags")
	if err != nil {
		panic(err)
	}
	flag.VisitAll(func(f *flag.Flag) {
		flagDirectory.RegisterMetric(
			f.Name,
			f.Value,
			flagUnit(f),
			f.Usage)
	})
}

func setFlagUnit(name string, unit units.Unit) {
	flagUnits[name] = unit
}

func defaultUnit(val interface{}) units.Unit {
	switch val.(type) {
	case time.Duration:
		return units.Second
	default:
		return units.None
	}
}

func flagUnit(f *flag.Flag) units.Unit {
	if unit, ok := flagUnits[f.Name]; ok {
		return unit
	}
	return defaultUnit(f.Value.(flag.Getter).Get())
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
