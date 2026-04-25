package uws1

// ExecutionRecords returns the last orchestrator execution snapshot captured
// on the document. The returned map is a defensive copy. It is not safe to call
// concurrently with Execute, DispatchTrigger, or SetRuntime unless the caller
// synchronizes access to the document.
func (d *Document) ExecutionRecords() map[string]ExecutionRecord {
	if d == nil || len(d.lastExecutionRecords) == 0 {
		return nil
	}
	out := make(map[string]ExecutionRecord, len(d.lastExecutionRecords))
	for key, record := range d.lastExecutionRecords {
		out[key] = cloneExecutionRecord(record)
	}
	return out
}

func (d *Document) setExecutionRecords(records map[string]ExecutionRecord) {
	if d == nil {
		return
	}
	if len(records) == 0 {
		d.lastExecutionRecords = nil
		return
	}
	d.lastExecutionRecords = make(map[string]ExecutionRecord, len(records))
	for key, record := range records {
		d.lastExecutionRecords[key] = cloneExecutionRecord(record)
	}
}
