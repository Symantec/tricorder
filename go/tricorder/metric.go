package tricorder

import (
	"errors"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	panicBadFunctionReturnTypes = "Functions must return either T or (T, error) where T is a primitive numeric type or a string."
	panicInvalidMetric          = "Invalid metric type."
	panicIncompatibleTypes      = "Wrong AsXXX function called on value."
)

var (
	root = newDirectory()
)

func newUnit(str string) (Unit, error) {
	switch str {
	case "None":
		return None, nil
	case "Millisecond":
		return Millisecond, nil
	case "Second":
		return Second, nil
	case "Celsius":
		return Celsius, nil
	default:
		return None, errors.New("Invalid unit name.")
	}
}

func (u Unit) _string() string {
	switch u {
	case None:
		return "None"
	case Millisecond:
		return "Millisecond"
	case Second:
		return "Second"
	case Celsius:
		return "Celsius"
	default:
		return "None"
	}
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

func newBucketerWithEndpoints(endpoints []float64) *Bucketer {
	endpointLen := len(endpoints)
	if endpointLen == 0 {
		panic("endpoints must have at least one element.")
	}
	pieces := make([]*bucketPiece, endpointLen+1)
	pieces[0] = &bucketPiece{First: true, End: endpoints[0]}
	for i := 1; i < endpointLen; i++ {
		pieces[i] = &bucketPiece{
			Start: endpoints[i-1], End: endpoints[i]}
	}
	pieces[endpointLen] = &bucketPiece{
		Last: true, Start: endpoints[endpointLen-1]}
	return &Bucketer{pieces: pieces}
}

func newBucketerWithScale(count int, start, scale float64) *Bucketer {
	if count < 2 || start <= 0.0 || scale <= 1.0 {
		panic("count >= 2 && start > 0.0 && scale > 1")
	}
	pieces := make([]*bucketPiece, count)
	current := start
	pieces[0] = &bucketPiece{First: true, End: current}
	for i := 1; i < count-1; i++ {
		next := current * scale
		pieces[i] = &bucketPiece{Start: current, End: next}
		current = next
	}
	pieces[count-1] = &bucketPiece{Last: true, Start: current}
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
		Count:     d.count,
		Breakdown: bdn,
	}

}

// valueType represents the type of a value
type valueType int

const (
	Int valueType = iota
	Uint
	Float
	String
	Dist
)

func (t valueType) String() string {
	switch t {
	case Int:
		return "int"
	case Uint:
		return "uint"
	case Float:
		return "float"
	case String:
		return "string"
	case Dist:
		return "distribution"
	default:
		return "none"
	}
}

// value represents the value of a metric.
type value struct {
	val          reflect.Value
	dist         *distribution
	valType      valueType
	funcArgCount int
}

func getPrimitiveType(t reflect.Type) valueType {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return Int
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return Uint
	case reflect.Float32, reflect.Float64:
		return Float
	case reflect.String:
		return String
	default:
		panic(panicInvalidMetric)
	}
}

func newValue(spec interface{}) *value {
	capDist, ok := spec.(*Distribution)
	if ok {
		return &value{dist: (*distribution)(capDist), valType: Dist}
	}
	dist, ok := spec.(*distribution)
	if ok {
		return &value{dist: dist, valType: Dist}
	}
	v := reflect.ValueOf(spec)
	t := v.Type()
	if t.Kind() == reflect.Func {
		funcArgCount := t.NumOut()

		// Our functions have to return one or two things
		if funcArgCount < 1 || funcArgCount > 2 {
			panic(panicBadFunctionReturnTypes)
		}
		if t.NumOut() == 2 {
			errorType := reflect.TypeOf((*error)(nil)).Elem()
			if !t.Out(1).Implements(errorType) {
				panic(panicBadFunctionReturnTypes)
			}
		}
		valType := getPrimitiveType(t.Out(0))
		return &value{
			val: v, valType: valType, funcArgCount: funcArgCount}
	}
	v = v.Elem()
	valType := getPrimitiveType(v.Type())
	return &value{val: v, valType: valType}
}

// Type returns the type of this value: Int, Float, Uint, String, or Dist
func (v *value) Type() valueType {
	return v.valType
}

func (v *value) evaluate() (reflect.Value, error) {
	if v.funcArgCount == 0 {
		return v.val, nil
	}
	result := v.val.Call(nil)
	if v.funcArgCount == 2 {
		return result[0], result[1].Interface().(error)
	}
	return result[0], nil
}

// AsXXX methods return this value as a type XX.
// AsXXX methods panic if this value is not of type XX.
// If this value represents a callback that can return an error,
// AsXXX methods  propagates the error from the callback.
func (v *value) AsInt() (result int64, err error) {
	if v.valType != Int {
		panic(panicIncompatibleTypes)
	}
	val, err := v.evaluate()
	if err != nil {
		return
	}
	result = val.Int()
	return
}

func (v *value) AsUint() (result uint64, err error) {
	if v.valType != Uint {
		panic(panicIncompatibleTypes)
	}
	val, err := v.evaluate()
	if err != nil {
		return
	}
	result = val.Uint()
	return
}

func (v *value) AsFloat() (result float64, err error) {
	if v.valType != Float {
		panic(panicIncompatibleTypes)
	}
	val, err := v.evaluate()
	if err != nil {
		return
	}
	result = val.Float()
	return
}

func (v *value) AsString() (result string, err error) {
	if v.valType != String {
		panic(panicIncompatibleTypes)
	}
	val, err := v.evaluate()
	if err != nil {
		return
	}
	result = val.String()
	return
}

// AsHtmlString returns this value as an html friendly string.
// AsHtmlString panics if this value does not represent a single value
// e.g a distribution.
// If this value represents a callback that can return an error,
// AsHtmlString propagates the error from the callback.
func (v *value) AsHtmlString() (result string, err error) {
	var val reflect.Value
	switch v.Type() {
	case Int, Uint, Float, String:
		val, err = v.evaluate()
		if err != nil {
			return
		}
		switch v.Type() {
		case Int:
			result = strconv.FormatInt(val.Int(), 10)
		case Uint:
			result = strconv.FormatUint(val.Uint(), 10)
		case Float:
			result = strconv.FormatFloat(val.Float(), 'f', -1, 64)
		case String:
			result = "\"" + val.String() + "\""
		default:
			panic("We should never get here!")
		}
		return
	default:
		panic(panicIncompatibleTypes)
	}
}

// AsDistribution returns this value as a Distribution.
// AsDistribution panics if this value does not represent a distribution
func (v *value) AsDistribution() *distribution {
	if v.valType != Dist {
		panic(panicIncompatibleTypes)
	}
	return v.dist
}

// metric represents a single metric.
type metric struct {
	// The description of the metric
	Description string
	// The unit of measurement
	Unit Unit
	// The value of the metric
	Value              *value
	enclosingListEntry *listEntry
}

// AbsPath returns the absolute path of this metric
func (m *metric) AbsPath() string {
	return "/" + m.enclosingListEntry.pathFrom(root).String()
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
	return "/" + d.enclosingListEntry.pathFrom(root).String()
}

// GetDirectory returns the directory with the given relative
// path or nil if no such directory exists.
func (d *directory) GetDirectory(relativePath string) *directory {
	return d.getDirectory(newPathSpec(relativePath))
}

// getMetric returns the metric with the given relative
// path or nil if no such metric exists.
func (d *directory) GetMetric(relativePath string) *metric {
	return d.getMetric(newPathSpec(relativePath))
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

func (d *directory) getMetric(path pathSpec) *metric {
	if path.Empty() {
		return nil
	}
	dir := d.getDirectory(path.Dir())
	if dir == nil {
		return nil
	}
	n := dir.contents[path.Base()]
	if n == nil {
		return nil
	}
	return n.Metric
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
	unit Unit,
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
		Value:       newValue(value)}
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
