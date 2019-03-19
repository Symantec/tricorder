package tricorder

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder/duration"
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
	panicSingleValueExpected    = "Trying to use an aggregate value in a single value context"
	panicNoAssignedUnit         = "Operation requires that distribution has assigned unit"
	panicListSubTypeChanging    = "Sub-type in list cannot change"
	panicBadValue               = "Value does not exist in distribution"
)

var (
	root          = newDirectory()
	intSizeInBits = int(unsafe.Sizeof(0)) * 8
	idGenerator   = newIdSequence()
)

func newIdSequence() chan int {
	result := make(chan int)
	go func() {
		id := 0
		for {
			result <- id
			id++
		}
	}()
	return result
}

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
	visitedRegions map[*region]time.Time
}

func newSession() *session {
	return &session{visitedRegions: make(map[*region]time.Time)}
}

// Visit indicates that caller is about to fetch metrics from a
// particular region. If this is the session's first time to visit the
// region and no other session is currently visiting it, then
// Visit calls the region's update function.
// Visit returns the timestamp of metrics in region r.
func (s *session) Visit(r *region) time.Time {
	if s.visitedRegions == nil {
		panic("Trying to visit with a closed session.")
	}
	result, ok := s.visitedRegions[r]
	if !ok {
		result = r.RLock()
		s.visitedRegions[r] = result
	}
	return result
}

// Close signals that the caller has retrieved all metrics for the
// particular request. In particular Close indicates that this session
// is finished visiting its regions.
// It is an error to use a session instance once it is closed.
func (s *session) Close() error {
	for visitedRegion := range s.visitedRegions {
		visitedRegion.RUnlock()
	}
	s.visitedRegions = nil
	return nil
}

func fixUpdateFunc(orig func()) func() time.Time {
	return func() time.Time {
		orig()
		return time.Now()
	}
}

type region struct {
	// The unique id. Immutable.
	id int
	// Channels to interact with event loop.
	sendCh              chan int
	receiveCh           chan time.Time
	updateFuncSendCh    chan func() time.Time
	updateFuncReceiveCh chan bool
	// Everything below here can only be accessed via the handleRequests
	// event loop.
	lockCount  int
	updateFunc func() time.Time
	updateTime time.Time
}

func newRegion(updateFunc func() time.Time) *region {
	result := &region{
		id:                  <-idGenerator,
		sendCh:              make(chan int),
		receiveCh:           make(chan time.Time),
		updateFuncSendCh:    make(chan func() time.Time),
		updateFuncReceiveCh: make(chan bool),
		updateFunc:          updateFunc}
	go func() {
		result.handleRequests()
	}()
	return result
}

func voidFunc() time.Time {
	return time.Now()
}

func newDefaultRegion() *region {
	return newRegion(voidFunc)
}

func (r *region) Id() int {
	return r.id
}

func (r *region) registerUpdateFunc(updateFunc func() time.Time) {
	r.updateFuncSendCh <- updateFunc
	<-r.updateFuncReceiveCh
}

func (r *region) handleLockUnlock(lockIncrement int) time.Time {
	prevLockCount := r.lockCount
	r.lockCount += lockIncrement
	if r.lockCount < 0 {
		panic("Lock count fell below 0")
	}
	// Update region for first lock holders
	if prevLockCount == 0 && r.lockCount > 0 {
		r.updateTime = r.updateFunc()
	}
	return r.updateTime
}

func (r *region) handleUpdateFunc(newFunc func() time.Time) bool {
	r.updateFunc = newFunc
	return true
}

func (r *region) handleRequests() {
	for {
		select {
		case in := <-r.sendCh:
			r.receiveCh <- r.handleLockUnlock(in)
		case in := <-r.updateFuncSendCh:
			r.updateFuncReceiveCh <- r.handleUpdateFunc(in)
		}
	}
}

func (r *region) RLock() time.Time {
	r.sendCh <- 1
	return <-r.receiveCh
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
	Min             float64
	Max             float64
	Average         float64
	Median          float64
	Sum             float64
	Count           uint64
	Generation      uint64
	IsNotCumulative bool
	TimeStamp       time.Time
	Breakdown       breakdown
}

// distribution represents a distribution of values same as Distribution
type distribution struct {
	pieces          []*bucketPiece
	isNotCumulative bool
	groupId         int
	// Protects all fields below it
	lock       sync.RWMutex
	unit       units.Unit
	unitSet    bool
	counts     []uint64
	total      float64
	min        float64
	max        float64
	count      uint64
	generation uint64
	timeStamp  time.Time
}

func newDistribution(bucketer *Bucketer, isNotCumulative bool) *distribution {
	// TODO: See about getting rid of newDistribution method.
	return newDistributionWithTimeStamp(
		bucketer, isNotCumulative, time.Now())
}

func newDistributionWithTimeStamp(
	bucketer *Bucketer,
	isNotCumulative bool,
	ts time.Time) *distribution {
	return &distribution{
		pieces:          bucketer.pieces,
		counts:          make([]uint64, len(bucketer.pieces)),
		isNotCumulative: isNotCumulative,
		timeStamp:       ts,
		groupId:         <-idGenerator,
	}
}

func (d *distribution) SetUnit(unit units.Unit) bool {
	d.lock.Lock()
	defer d.lock.Unlock()
	// Always set unit on first call
	if !d.unitSet {
		d.unitSet = true
		d.unit = unit
		return true
	}
	if unit == d.unit {
		// unit already set to what we want just return success
		return true
	}
	// Oops, unit already set to something other than what we want
	// return failure
	return false
}

func (d *distribution) GroupId() int {
	return d.groupId
}

func (d *distribution) Sum() float64 {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.total
}

func (d *distribution) Count() uint64 {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.count
}

func (d *distribution) Add(value interface{}) {
	d.AddWithTs(value, time.Now())
}

func (d *distribution) AddWithTs(value interface{}, ts time.Time) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.add(d.valueToFloat(value), ts)
}

func (d *distribution) Update(oldValue, newValue interface{}) {
	d.UpdateWithTs(oldValue, newValue, time.Now())
}

func (d *distribution) UpdateWithTs(
	oldValue, newValue interface{}, ts time.Time) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.update(
		d.valueToFloat(oldValue),
		d.valueToFloat(newValue),
		ts)
}

func (d *distribution) Remove(valueToBeRemoved interface{}) {
	d.RemoveWithTs(valueToBeRemoved, time.Now())
}

func (d *distribution) RemoveWithTs(
	valueToBeRemoved interface{}, ts time.Time) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.remove(d.valueToFloat(valueToBeRemoved), ts)
}

func (d *distribution) add(value float64, ts time.Time) {
	idx := findDistributionIndex(d.pieces, value)
	d.timeStamp = ts
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
	d.generation++
}

func decrementCount(count *uint64) {
	if *count == 0 {
		panic(panicBadValue)
	}
	(*count)--
}

func (d *distribution) update(
	oldValue, newValue float64, ts time.Time) {
	if !d.isNotCumulative {
		panic("Cannot call update on a cumulative distribution.")
	}
	if d.count == 0 {
		panic("Can't call update on an empty distribution.")
	}
	d.timeStamp = ts
	oldIdx := findDistributionIndex(d.pieces, oldValue)
	newIdx := findDistributionIndex(d.pieces, newValue)
	d.counts[newIdx]++
	decrementCount(&d.counts[oldIdx])
	d.total += (newValue - oldValue)
	if newValue < d.min {
		d.min = newValue
	} else if newValue > d.max {
		d.max = newValue
	}
	d.generation++
}

func (d *distribution) remove(
	valueToBeRemoved float64, ts time.Time) {
	if !d.isNotCumulative {
		panic("Cannot call remove on a cumulative distribution.")
	}
	if d.count == 0 {
		panic("Can't call remove on an empty distribution.")
	}
	d.timeStamp = ts
	idx := findDistributionIndex(d.pieces, valueToBeRemoved)
	decrementCount(&d.counts[idx])
	d.total -= valueToBeRemoved
	d.count--
	d.generation++
}

func (d *distribution) valueToFloat(value interface{}) float64 {
	if !d.unitSet {
		panic(panicNoAssignedUnit)
	}
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
			TimeStamp: d.timeStamp,
		}
	}
	return &snapshot{
		Min:             d.min,
		Max:             d.max,
		Average:         d.total / float64(d.count),
		Median:          d.calculateMedian(),
		Sum:             d.total,
		Count:           d.count,
		Generation:      d.generation,
		IsNotCumulative: d.isNotCumulative,
		TimeStamp:       d.timeStamp,
		Breakdown:       bdn,
	}

}

type listType struct {
	groupId   int
	subType   types.Type
	lock      sync.RWMutex
	aSlice    reflect.Value
	timeStamp time.Time
}

func copySlice(value reflect.Value) reflect.Value {
	result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
	reflect.Copy(result, value)
	return result
}

// Converts []int or []uint slice to a slice having element type t.
// If value is an []int, t has to be types.Int32 or types.Int64.
// If value is an []uint, t has to be types.Uint32 or types.Uint64.
func convertFromIntOrUintSlice(
	value reflect.Value, t types.Type) reflect.Value {
	var newType reflect.Type
	switch t {
	case types.Int32:
		var unused []int32
		newType = reflect.TypeOf(unused)
	case types.Int64:
		var unused []int64
		newType = reflect.TypeOf(unused)
	case types.Uint32:
		var unused []uint32
		newType = reflect.TypeOf(unused)
	case types.Uint64:
		var unused []uint64
		newType = reflect.TypeOf(unused)
	default:
		panic("t is not compatible with int or uint")
	}
	length := value.Len()
	defensiveCopy := reflect.MakeSlice(newType, length, length)
	for i := 0; i < length; i++ {
		defensiveCopy.Index(i).Set(
			reflect.ValueOf(
				valueToInterface(
					value.Index(i),
					t,
					false)))
	}
	return defensiveCopy
}

// Returns aSlice as a value and its type. If sliceIsMutable is true,
// always makes a defensive copy of aSlice. If sliceIsMutable is false,
// may or may not make a defensive copy of aSlice. If aSlice is an []int
// or []uint, returned value will be size specific, e.g []int64.
func asSliceValue(aSlice interface{}, sliceIsMutable bool) (
	value reflect.Value, t types.Type) {
	value = reflect.ValueOf(aSlice)
	if value.Kind() != reflect.Slice {
		panic("Slice expected")
	}
	elemType := value.Type().Elem()
	t, isPtr := mustGetPrimitiveType(elemType)
	if isPtr {
		panic("Lists cannot contain slices of pointrs.")
	}
	elemKind := elemType.Kind()
	if elemKind == reflect.Int || elemKind == reflect.Uint {
		value = convertFromIntOrUintSlice(value, t)
	} else if sliceIsMutable {
		value = copySlice(value)
	}
	return
}

func newListWithTimeStamp(
	aSlice interface{},
	sliceIsMutable bool,
	ts time.Time) *listType {
	value, subType := asSliceValue(aSlice, sliceIsMutable)
	return &listType{
		groupId:   <-idGenerator,
		subType:   subType,
		aSlice:    value,
		timeStamp: ts}
}

func (l *listType) ChangeWithTimeStamp(
	aSlice interface{},
	sliceIsMutable bool,
	ts time.Time) {
	value, subType := asSliceValue(aSlice, sliceIsMutable)
	if subType != l.subType {
		panic(panicListSubTypeChanging)
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	l.aSlice = value
	l.timeStamp = ts
}

func (l *listType) get() (value reflect.Value, ts time.Time) {
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.aSlice, l.timeStamp
}

func (l *listType) TextStrings(unit units.Unit) []string {
	value, _ := l.get()
	result := make([]string, value.Len())
	for i := range result {
		result[i] = valueToTextString(
			value.Index(i), l.subType, unit, false)
	}
	return result
}

func (l *listType) HtmlStrings(unit units.Unit) []string {
	value, _ := l.get()
	result := make([]string, value.Len())
	for i := range result {
		result[i] = valueToHtmlString(
			value.Index(i), l.subType, unit, false)
	}
	return result
}

// Callers must never modify returned slice in place.
func (l *listType) AsSlice() (interface{}, time.Time) {
	value, ts := l.get()
	return value.Interface(), ts
}

func (l *listType) GroupId() int {
	return l.groupId
}

func (l *listType) SubType() types.Type {
	return l.subType
}

// value represents the value of a metric.
type value struct {
	val           reflect.Value
	region        *region
	dist          *distribution
	alist         *listType
	valType       types.Type
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
// types.Type, and true if t is a pointer to that type
// or false otherwise.
// If t is not supported, returns ok = false.
func getPrimitiveType(t reflect.Type) (
	outType types.Type, isPtr, ok bool) {
	switch t {
	case timePtrType:
		return types.GoTime, true, true
	case timeType:
		return types.GoTime, false, true
	case durationType:
		return types.GoDuration, false, true
	default:
		switch t.Kind() {
		case reflect.Bool:
			return types.Bool, false, true
		case reflect.Int:
			switch intSizeInBits {
			case 32:
				return types.Int32, false, true
			case 64:
				return types.Int64, false, true
			default:
				return
			}
		case reflect.Int8:
			return types.Int8, false, true
		case reflect.Int16:
			return types.Int16, false, true
		case reflect.Int32:
			return types.Int32, false, true
		case reflect.Int64:
			return types.Int64, false, true
		case reflect.Uint:
			switch intSizeInBits {
			case 32:
				return types.Uint32, false, true
			case 64:
				return types.Uint64, false, true
			default:
				return
			}
		case reflect.Uint8:
			return types.Uint8, false, true
		case reflect.Uint16:
			return types.Uint16, false, true
		case reflect.Uint32:
			return types.Uint32, false, true
		case reflect.Uint64:
			return types.Uint64, false, true
		case reflect.Float32:
			return types.Float32, false, true
		case reflect.Float64:
			return types.Float64, false, true
		case reflect.String:
			return types.String, false, true
		default:
			return
		}
	}
}

// like getPrimitiveType but panics if t is not supported
func mustGetPrimitiveType(t reflect.Type) (
	outType types.Type, isPtr bool) {
	outType, isPtr, ok := getPrimitiveType(t)
	if !ok {
		panic(panicInvalidMetric)
	}
	return
}

func newValueForDist(dist *distribution, unit units.Unit) (
	*value, error) {
	if !dist.SetUnit(unit) {
		return nil, ErrWrongUnit
	}
	return &value{dist: dist, unit: unit, valType: types.Dist}, nil

}

// unit parameter only used if spec is a *Distribution
// In that case, it sets the unit of the *Distribution in place.
func newValue(spec interface{}, region *region, unit units.Unit) (
	*value, error) {
	if someDist, ok := spec.(*NonCumulativeDistribution); ok {
		dist := (*distribution)(someDist)
		return newValueForDist(dist, unit)
	}
	if someDist, ok := spec.(*CumulativeDistribution); ok {
		dist := (*distribution)(someDist)
		return newValueForDist(dist, unit)
	}
	if dist, ok := spec.(*distribution); ok {
		return newValueForDist(dist, unit)
	}
	if someList, ok := spec.(*List); ok {
		alist := (*listType)(someList)
		return &value{alist: alist, unit: unit, valType: types.List}, nil
	}
	if alist, ok := spec.(*listType); ok {
		return &value{alist: alist, unit: unit, valType: types.List}, nil
	}
	flagValue, ok := spec.(flag.Value)
	if ok {
		flagGetter := toFlagGetter(flagValue)
		t := reflect.ValueOf(flagGetter.Get()).Type()
		valType, isValAPointer, ok := getPrimitiveType(t)
		var valFunc reflect.Value
		if !ok {
			valType = types.String
			isValAPointer = false
			valFunc = reflect.ValueOf(flagGetter.String)
		} else {
			valFunc = reflect.ValueOf(flagGetter.Get)
		}
		return &value{
			val:           valFunc,
			unit:          unit,
			region:        region,
			valType:       valType,
			isfunc:        true,
			isValAPointer: isValAPointer}, nil
	}
	v := reflect.ValueOf(spec)
	t := v.Type()
	if t.Kind() == reflect.Func {
		funcArgCount := t.NumOut()

		// Our functions have to return exactly one thing
		if funcArgCount != 1 {
			panic(panicBadFunctionReturnTypes)
		}
		valType, isValAPointer, ok := getPrimitiveType(t.Out(0))
		if !ok {
			return nil, ErrWrongType
		}
		return &value{
			val:           v,
			unit:          unit,
			region:        region,
			valType:       valType,
			isfunc:        true,
			isValAPointer: isValAPointer}, nil
	}
	v = v.Elem()
	valType, isValAPointer, ok := getPrimitiveType(v.Type())
	if !ok {
		return nil, ErrWrongType
	}
	return &value{
		val:           v,
		unit:          unit,
		region:        region,
		valType:       valType,
		isValAPointer: isValAPointer}, nil
}

// Type returns the type of this value: Int, Float, Uint, String, or Dist
func (v *value) Type() types.Type {
	return v.valType
}

// SubType returns the sub type of this value when this value is a
// list or types.Unknown if this value is not a list.
func (v *value) SubType() types.Type {
	if v.valType == types.List {
		return v.alist.SubType()
	}
	return types.Unknown
}

// Unit returns the unit of this value.
func (v *value) Unit() units.Unit {
	return v.unit
}

// Bits returns the size in bits if the type is an Int, Uint, or Float.
// Otherwise Bits returns 0. If this value represents a list,
// returns the size in bits of the sub type.
func (v *value) Bits() int {
	if v.valType == types.List {
		return v.alist.SubType().Bits()
	}
	return v.valType.Bits()
}

func (v *value) canEvaluate() bool {
	return v.val.IsValid()
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

// AsBool methods return this value as a type XX.
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
	if !v.valType.IsInt() {
		panic(panicIncompatibleTypes)
	}
	return v.evaluate(s).Int()
}

func (v *value) AsUint(s *session) uint64 {
	if !v.valType.IsUint() {
		panic(panicIncompatibleTypes)
	}
	return v.evaluate(s).Uint()
}

func (v *value) IsInfNaN(s *session) bool {
	if !v.valType.IsFloat() {
		return false
	}
	val := v.evaluate(s).Float()
	return math.IsNaN(val) || math.IsInf(val, 0)
}

func (v *value) AsFloat(s *session) float64 {
	if !v.valType.IsFloat() {
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

func valueToTime(
	value reflect.Value, isValueAPointer bool) (result time.Time) {
	if isValueAPointer {
		p := value.Interface().(*time.Time)
		if p == nil {
			return
		}
		return *p
	}
	return value.Interface().(time.Time)
}

func valueToGoDuration(value reflect.Value) time.Duration {
	return time.Duration(value.Int())
}

func timeToDuration(t time.Time) (result duration.Duration) {
	if t.IsZero() {
		return
	}
	return duration.SinceEpoch(t)
}

func (v *value) AsTime(s *session) (result time.Time) {
	if v.valType != types.GoTime {
		panic(panicIncompatibleTypes)
	}
	return valueToTime(v.evaluate(s), v.isValAPointer)
}

func (v *value) AsGoDuration(s *session) time.Duration {
	if v.valType != types.GoDuration {
		panic(panicIncompatibleTypes)
	}
	return valueToGoDuration(v.evaluate(s))
}

func (v *value) AsDuration(s *session) duration.Duration {
	if v.valType == types.GoTime {
		return timeToDuration(v.AsTime(s))
	}
	if v.valType == types.GoDuration {
		return duration.New(v.AsGoDuration(s))
	}
	panic(panicIncompatibleTypes)
}

func valueToInterface(
	value reflect.Value,
	t types.Type,
	isValueAPointer bool) interface{} {
	// Int32, Int64, Uint32, and Uint64 cases are necessary in case
	// client passed an int or uint pointer to RegisterMetric.
	// If we were to just call v.evaluate(s).Interface() we would return
	// that plain int or uint instead of a sized int or uint and violate
	// the contract of the API which specifies that the value is always
	// a sized int or uint.
	switch {
	case t == types.Int32 && !isValueAPointer:
		return int32(value.Int())
	case t == types.Int64 && !isValueAPointer:
		return int64(value.Int())
	case t == types.Uint32 && !isValueAPointer:
		return uint32(value.Uint())
	case t == types.Uint64 && !isValueAPointer:
		return uint64(value.Uint())
	case t == types.GoTime:
		return valueToTime(value, isValueAPointer)
	case t == types.GoDuration:
		return valueToGoDuration(value)
	case !isValueAPointer:
		return value.Interface()
	default:
		panic(panicIncompatibleTypes)
	}

}

func (v *value) AsInterface(s *session) (result interface{}) {
	if !v.canEvaluate() {
		panic(panicSingleValueExpected)
	}
	return valueToInterface(v.evaluate(s), v.valType, v.isValAPointer)
}

// RegionId returns the region id for the timestamps of this value.
// RegionId panics if called on an aggregate value such as a distrubtion or
// list since they are updated continually.
func (v *value) RegionId() int {
	if v.region == nil {
		panic("Cannot all RegionId on a distribution value.")
	}
	return v.region.Id()
}

// TimeStamp returns the timestamp of the value fetched with the given session.
// s must be non-nil and must be the same session passed to an AsXXX method
// to fetch the value. The caller may call this method and the AsXXX method
// in any order as long as it passes the same session to both.
// TimeStamp panics if called on an aggregat value such as a  distribution
// or list since they are updated continually and have no timestamp.
func (v *value) TimeStamp(s *session) time.Time {
	if v.region == nil {
		panic("Cannot call TimeStamp on an aggregate value.")
	}
	return s.Visit(v.region)
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
	metric.Kind = t
	metric.SubType = v.SubType()
	metric.Bits = v.Bits()
	metric.Unit = v.Unit()
	switch t {
	case types.Dist:
		dist := v.AsDistribution()
		snapshot := dist.Snapshot()
		metric.Value = &messages.Distribution{
			Min:             snapshot.Min,
			Max:             snapshot.Max,
			Average:         snapshot.Average,
			Median:          snapshot.Median,
			Sum:             snapshot.Sum,
			Count:           snapshot.Count,
			Generation:      snapshot.Generation,
			IsNotCumulative: snapshot.IsNotCumulative,
			Ranges:          asRanges(snapshot.Breakdown)}
		metric.GroupId = dist.GroupId()
		metric.TimeStamp = snapshot.TimeStamp
	case types.List:
		alist := v.AsList()
		metric.Value, metric.TimeStamp = alist.AsSlice()
		metric.GroupId = alist.GroupId()
	default:
		if s == nil {
			s = newSession()
			defer s.Close()
		}
		metric.Value = v.AsInterface(s)
		metric.GroupId = v.RegionId()
		metric.TimeStamp = v.TimeStamp(s)
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

func addSeparators(orig string, separator rune, digitsBetween int) string {
	s := ([]byte)(orig)
	start := bytes.IndexFunc(s, func(r rune) bool {
		return r >= '0' && r <= '9'
	})
	end := bytes.IndexFunc(s[start:], func(r rune) bool {
		return r < '0' || r > '9'
	}) + start
	if end < start {
		end = len(s)
	}
	digits := bytes.Runes(s[start:end])
	buffer := &bytes.Buffer{}
	buffer.Write(s[:start])
	dlen := len(digits)
	for i, dig := range digits {
		digitsLeft := dlen - i
		if i > 0 && digitsLeft%digitsBetween == 0 {
			buffer.WriteRune(separator)
		}
		buffer.WriteRune(dig)
	}
	buffer.Write(s[end:])
	return buffer.String()
}

func valueToTextString(
	value reflect.Value,
	t types.Type,
	u units.Unit,
	isValueAPointer bool) string {
	switch {
	case t == types.Bool:
		if value.Bool() {
			return "true"
		}
		return "false"
	case t.IsInt() && !isValueAPointer:
		return strconv.FormatInt(value.Int(), 10)
	case t.IsUint() && !isValueAPointer:
		return strconv.FormatUint(value.Uint(), 10)
	case t == types.Float32 && !isValueAPointer:
		return strconv.FormatFloat(value.Float(), 'f', -1, 32)
	case t == types.Float64 && !isValueAPointer:
		return strconv.FormatFloat(value.Float(), 'f', -1, 64)
	case t == types.String && !isValueAPointer:
		return "\"" + value.String() + "\""
	case t == types.GoTime:
		return timeToDuration(
			valueToTime(
				value, isValueAPointer)).StringUsingUnits(u)
	case t == types.GoDuration && !isValueAPointer:
		return duration.New(
			valueToGoDuration(value)).StringUsingUnits(u)
	default:
		panic(panicIncompatibleTypes)
	}
}

// AsTextString returns this value as a text friendly string.
// AsTextString panics if this value does not represent a single value.
// For example, AsTextString panics if this value represents a distribution.
// If caller passes a nil session, AsTextString creates its own internally.
func (v *value) AsTextString(s *session) string {
	if !v.canEvaluate() {
		panic(panicSingleValueExpected)
	}
	return valueToTextString(
		v.evaluate(s), v.Type(), v.unit, v.isValAPointer)
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

func valueToHtmlString(
	value reflect.Value,
	t types.Type,
	u units.Unit,
	isValueAPointer bool) string {
	switch {
	case t.IsInt() && !isValueAPointer:
		switch u {
		case units.Byte:
			return iCompactForm(
				value.Int(), 1024, byteSuffixes)
		case units.BytePerSecond:
			return iCompactForm(
				value.Int(), 1024, bytePerSecondSuffixes)
		case units.None:
			return addSeparators(
				valueToTextString(value, t, u, isValueAPointer), ',', 3)
		default:
			return iCompactForm(
				value.Int(), 1000, suffixes)
		}
	case t.IsUint() && !isValueAPointer:
		switch u {
		case units.Byte:
			return uCompactForm(
				value.Uint(), 1024, byteSuffixes)
		case units.BytePerSecond:
			return uCompactForm(
				value.Uint(), 1024, bytePerSecondSuffixes)
		case units.None:
			return addSeparators(
				valueToTextString(value, t, u, isValueAPointer), ',', 3)
		default:
			return uCompactForm(
				value.Uint(), 1000, suffixes)
		}
	case t == types.GoDuration && !isValueAPointer:
		d := duration.New(valueToGoDuration(value))
		if d.IsNegative() {
			return valueToTextString(
				value, t, u, isValueAPointer)
		}
		return d.PrettyFormat()
	case t == types.GoTime:
		t := valueToTime(value, isValueAPointer).UTC()
		return t.Format("2006-01-02T15:04:05.999999999Z")
	default:
		return valueToTextString(value, t, u, isValueAPointer)
	}
}

// AsHtmlString returns this value as an html friendly string.
// AsHtmlString panics if this value does not represent a single value.
// For example, AsHtmlString panics if this value represents a distribution.
// If caller passes a nil session, AsHtmlString creates its own internally.
func (v *value) AsHtmlString(s *session) string {
	if !v.canEvaluate() {
		panic(panicSingleValueExpected)
	}
	return valueToHtmlString(
		v.evaluate(s), v.Type(), v.unit, v.isValAPointer)
}

// AsDistribution returns this value as a Distribution.
// AsDistribution panics if this value does not represent a distribution
func (v *value) AsDistribution() *distribution {
	if v.valType != types.Dist {
		panic(panicIncompatibleTypes)
	}
	return v.dist
}

func (v *value) AsList() *listType {
	if v.valType != types.List {
		panic(panicIncompatibleTypes)
	}
	return v.alist
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

// InitJsonMetric initializes 'metric' for JSON with this instance
func (m *metric) InitJsonMetric(s *session, metric *messages.Metric) {
	*metric = messages.Metric{
		Path: m.AbsPath(), Description: m.Description}
	m.value.UpdateJsonMetric(s, metric)
}

// UpdateJsonMetric is a synonym for InitJsonMetric. Here to prevent
// client from unintenionally calling value.UpdateJsonMetric which does
// only a partial initialization.
func (m *metric) UpdateJsonMetric(s *session, metric *messages.Metric) {
	m.InitJsonMetric(s, metric)
}

// InitRpcMetric initializes 'metric' for GoRPC with this instance
func (m *metric) InitRpcMetric(s *session, metric *messages.Metric) {
	*metric = messages.Metric{
		Path: m.AbsPath(), Description: m.Description}
	m.value.UpdateRpcMetric(s, metric)
}

// UpdateRpcMetric is a synonym for InitRpcMetric. Here to prevent
// client from unintenionally calling value.UpdateRpcMetric which does
// only a partial initialization.
func (m *metric) UpdateRpcMetric(s *session, metric *messages.Metric) {
	m.InitRpcMetric(s, metric)
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

func (n *listEntry) parentDir() *directory {
	if n.parent == nil {
		return root
	}
	return n.parent.Directory
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
	enclosingListEntry *listEntry
	// lock locks only the contents map itself.
	lock     sync.RWMutex
	contents map[string]*listEntry
}

func newDirectory() *directory {
	return &directory{contents: make(map[string]*listEntry)}
}

// List lists the contents of this directory in lexographical order by name.
func (d *directory) List() []*listEntry {
	return sortListEntries(d.listUnsorted())
}

// AbsPath returns the absolute path of this directory
func (d *directory) AbsPath() string {
	return d.enclosingListEntry.absPath()
}

// Parent returns the parent directory of this directory or nil if this
// directory is the root directory.
func (d *directory) Parent() *directory {
	if d.enclosingListEntry == nil {
		return nil
	}
	return d.enclosingListEntry.parentDir()
}

func (d *directory) IsRoot() bool {
	return d.enclosingListEntry == nil
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

// s is always non-nil
func collect(m *metric, s *session, coll metricsCollector) error {
	if m.IsInfNaN(s) {
		return nil
	}
	return coll.Collect(m, s)
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
		return collect(m, s, collector)
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

// GetAllMetrics does a depth first traversal of this directory to
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
			err = collect(entry.Metric, s, collector)
		}
		if err != nil {
			return
		}
	}
	return
}

func (d *directory) listUnsorted() []*listEntry {
	d.lock.RLock()
	defer d.lock.RUnlock()
	result := make([]*listEntry, len(d.contents))
	idx := 0
	for _, n := range d.contents {
		result[idx] = n
		idx++
	}
	return result
}

func (d *directory) getListEntry(name string) *listEntry {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.contents[name]
}

func (d *directory) removeListEntry(name string) {
	d.lock.Lock()
	defer d.lock.Unlock()
	delete(d.contents, name)
}

func (d *directory) removeChildDirectory(child *directory) {
	name := child.enclosingListEntry.Name
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.contents[name] == child.enclosingListEntry {
		delete(d.contents, name)
	}
}

func (d *directory) getDirectoryAndError(path pathSpec) (
	*directory, error) {
	result := d
	for _, part := range path {
		n := result.getListEntry(part)
		if n == nil {
			return nil, ErrNotFound
		}
		if n.Directory == nil {
			return nil, ErrPathInUse
		}
		result = n.Directory
	}
	return result, nil
}

func (d *directory) getDirectory(path pathSpec) (result *directory) {
	result = d
	for _, part := range path {
		n := result.getListEntry(part)
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
	n := dir.getListEntry(path.Base())
	if n == nil {
		return nil, nil
	}
	return n.Directory, n.Metric
}

func (d *directory) createDirIfNeeded(name string) (*directory, error) {
	d.lock.Lock()
	defer d.lock.Unlock()
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
	d.lock.Lock()
	defer d.lock.Unlock()
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
	avalue, err := newValue(value, region, unit)
	if err != nil {
		return
	}
	metric := &metric{
		Description: description,
		value:       avalue}
	return current.storeMetric(path.Base(), metric)
}

func (d *directory) unregisterDirectory() {
	if d.IsRoot() {
		return
	}
	d.Parent().removeChildDirectory(d)
}

func (d *directory) unregisterPath(path pathSpec) {
	if path.Empty() {
		return
	}
	dir := d.getDirectory(path.Dir())
	if dir == nil {
		return
	}
	dir.removeListEntry(path.Base())
}

type flagValueToGetterType struct {
	flag.Value
}

func (f *flagValueToGetterType) Get() interface{} {
	return f.String()
}

func toFlagGetter(value flag.Value) flag.Getter {
	result, ok := value.(flag.Getter)
	if ok {
		return result
	}
	return &flagValueToGetterType{value}
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
	return defaultUnit(toFlagGetter(f.Value).Get())
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

func readMyMetrics(path string) (result messages.MetricList) {
	// Always returns nil error since rpcMetricsCollector.Collect
	// always returns nil
	root.GetAllMetricsByPath(
		path, (*rpcMetricsCollector)(&result), nil)
	return
}
