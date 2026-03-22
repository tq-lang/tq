package input

import "strings"

// pushTopKey manages the path when entering a new key-value pair.
func (tr *TOONTokenReader) pushTopKey(indent int, key string) {
	tr.popPreviousTopKey(indent)
	tr.path = append(tr.path, key)
	if tr.isTopLevel(indent) {
		tr.hasTopKey = true
	}
}

func (tr *TOONTokenReader) isTopLevel(indent int) bool {
	return indent == 0 && len(tr.stack) == 0
}

func (tr *TOONTokenReader) pushObjectContainer(indent int) {
	tr.stack = append(tr.stack, containerInfo{
		kind:   containerObject,
		indent: indent + 1,
	})
}

func (tr *TOONTokenReader) emitPrimitive(token string) error {
	value, err := decodePrimitiveToken(token)
	if err != nil {
		return err
	}
	tr.emitLeaf(value)
	return nil
}

func (tr *TOONTokenReader) processArrayHeader(indent int, header toonParsedHeader) error {
	tr.popPreviousTopKey(indent)
	tr.pushHeaderKey(indent, header.key)

	delimiter := header.delimiter.toRune()
	if header.inlineValues != "" {
		return tr.emitInlineArray(header.inlineValues, delimiter)
	}
	tr.pushArrayContainer(indent, header, delimiter)
	return nil
}

func (tr *TOONTokenReader) popPreviousTopKey(indent int) {
	if tr.isTopLevel(indent) && tr.hasTopKey && len(tr.path) > 0 {
		tr.path = tr.path[:len(tr.path)-1]
	}
}

func (tr *TOONTokenReader) pushHeaderKey(indent int, key string) {
	if key == "" {
		return
	}
	tr.path = append(tr.path, key)
	if tr.isTopLevel(indent) {
		tr.hasTopKey = true
	}
}

func (tr *TOONTokenReader) pushArrayContainer(indent int, header toonParsedHeader, delimiter rune) {
	tr.path = append(tr.path, 0)
	if len(header.fields) > 0 {
		tr.pushTabularContainer(indent, header.fields, delimiter)
	} else {
		tr.pushListContainer(indent)
	}
}

func (tr *TOONTokenReader) pushTabularContainer(indent int, fields []string, delimiter rune) {
	tr.stack = append(tr.stack, containerInfo{
		kind:      containerTabularArray,
		indent:    indent + 1,
		fields:    fields,
		delimiter: delimiter,
		index:     0,
	})
}

func (tr *TOONTokenReader) pushListContainer(indent int) {
	tr.stack = append(tr.stack, containerInfo{
		kind:   containerListArray,
		indent: indent + 1,
		index:  0,
	})
}

// emitInlineArray emits all values of an inline array like "[3]: a,b,c".
func (tr *TOONTokenReader) emitInlineArray(values string, delimiter rune) error {
	raw, err := splitInlineValues(values, delimiter)
	if err != nil {
		return err
	}
	if err := tr.emitIndexedValues(raw); err != nil {
		return err
	}
	tr.emitArrayClose(len(raw))
	return nil
}

func (tr *TOONTokenReader) emitIndexedValues(raw []string) error {
	for i, token := range raw {
		tr.path = append(tr.path, i)
		if err := tr.emitPrimitive(token); err != nil {
			return err
		}
		tr.path = tr.path[:len(tr.path)-1]
	}
	return nil
}

func (tr *TOONTokenReader) emitArrayClose(count int) {
	if count > 0 {
		tr.path = append(tr.path, count-1)
		tr.emitTruncate()
		tr.path = tr.path[:len(tr.path)-1]
	} else {
		tr.emit([]any{tr.copyPath(), []any{}})
	}
}

func (tr *TOONTokenReader) processListItem(indent int, content string, listIdx int) error {
	top := &tr.stack[listIdx]
	itemContent := strings.TrimSpace(content[1:]) // skip '-'
	tr.path[len(tr.path)-1] = top.index

	if itemContent == "" {
		return tr.emitEmptyListItem(top)
	}
	return tr.processListContent(indent, itemContent, listIdx, top)
}

func (tr *TOONTokenReader) emitEmptyListItem(top *containerInfo) error {
	tr.emit([]any{tr.copyPath(), map[string]any{}})
	top.index++
	return nil
}

func (tr *TOONTokenReader) processListContent(indent int, content string, listIdx int, top *containerInfo) error {
	if handled, err := tr.tryInlineSubArray(content, listIdx); handled || err != nil {
		return err
	}
	if isKeyValue(content) {
		return tr.processListObjectItem(indent, content, listIdx)
	}
	return tr.processListScalar(content, top)
}

func (tr *TOONTokenReader) tryInlineSubArray(content string, listIdx int) (bool, error) {
	if !strings.HasPrefix(content, "[") {
		return false, nil
	}
	return tr.parseAndEmitInlineSubArray(content, listIdx)
}

func (tr *TOONTokenReader) parseAndEmitInlineSubArray(content string, listIdx int) (bool, error) {
	header, ok, err := tryParseHeader(content)
	if err != nil || !ok {
		return false, err
	}
	return true, tr.processInlineArrayInList(header, listIdx)
}

func (tr *TOONTokenReader) processListObjectItem(indent int, content string, listIdx int) error {
	count, err := tr.emitListObjectKV(content)
	if err != nil {
		return err
	}
	tr.pushObjectContainerWithCount(indent, count)
	tr.stack[listIdx].index++
	return nil
}

func (tr *TOONTokenReader) emitListObjectKV(content string) (int, error) {
	key, rest, err := splitKeyValue(content)
	if err != nil {
		return 0, err
	}
	tr.path = append(tr.path, key)
	return tr.emitOptionalValue(rest)
}

func (tr *TOONTokenReader) emitOptionalValue(rest string) (int, error) {
	if rest == "" {
		return 0, nil
	}
	return 1, tr.emitPrimitive(rest)
}

func (tr *TOONTokenReader) pushObjectContainerWithCount(indent, childCount int) {
	tr.stack = append(tr.stack, containerInfo{
		kind:       containerObject,
		indent:     indent + 1,
		childCount: childCount,
	})
}

func (tr *TOONTokenReader) processListScalar(content string, top *containerInfo) error {
	if err := tr.emitPrimitive(content); err != nil {
		return err
	}
	top.index++
	return nil
}

func (tr *TOONTokenReader) processInlineArrayInList(header toonParsedHeader, listIdx int) error {
	if header.inlineValues != "" {
		if err := tr.emitInlineArray(header.inlineValues, header.delimiter.toRune()); err != nil {
			return err
		}
	}
	tr.stack[listIdx].index++
	return nil
}

func (tr *TOONTokenReader) processTabularRow(_ int, content string, top *containerInfo) error {
	tr.path[len(tr.path)-1] = top.index
	raw, err := splitInlineValues(strings.TrimSpace(content), top.delimiter)
	if err != nil {
		return err
	}
	return tr.emitTabularFields(top, raw)
}

func (tr *TOONTokenReader) emitTabularFields(top *containerInfo, raw []string) error {
	if err := tr.emitFieldValues(top.fields, raw); err != nil {
		return err
	}
	tr.emitTabularRowClose(top.fields)
	top.index++
	return nil
}

func (tr *TOONTokenReader) emitFieldValues(fields, raw []string) error {
	for i, field := range fields {
		if i >= len(raw) {
			break
		}
		if err := tr.emitNamedField(field, raw[i]); err != nil {
			return err
		}
	}
	return nil
}

func (tr *TOONTokenReader) emitNamedField(name, token string) error {
	tr.path = append(tr.path, name)
	err := tr.emitPrimitive(token)
	tr.path = tr.path[:len(tr.path)-1]
	return err
}

func (tr *TOONTokenReader) emitTabularRowClose(fields []string) {
	if len(fields) == 0 {
		return
	}
	lastField := fields[len(fields)-1]
	tr.path = append(tr.path, lastField)
	tr.emitTruncate()
	tr.path = tr.path[:len(tr.path)-1]
}
