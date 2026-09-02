package label

import "fmt"

func (d *Document) AddElement(element Element) error {
	if d == nil {
		return fmt.Errorf("nil document")
	}
	if element.ID == "" {
		return fmt.Errorf("element id is required")
	}
	if _, ok := d.ElementByID(element.ID); ok {
		return fmt.Errorf("element %q already exists", element.ID)
	}
	d.Elements = append(d.Elements, element)
	return nil
}

func (d *Document) DeleteElement(id string) bool {
	if d == nil {
		return false
	}
	for i, element := range d.Elements {
		if element.ID != id {
			continue
		}
		d.Elements = append(d.Elements[:i], d.Elements[i+1:]...)
		return true
	}
	return false
}

func (d *Document) UpdateElement(element Element) bool {
	if d == nil {
		return false
	}
	for i, current := range d.Elements {
		if current.ID != element.ID {
			continue
		}
		d.Elements[i] = element
		return true
	}
	return false
}

func (d Document) ElementByID(id string) (Element, bool) {
	for _, element := range d.Elements {
		if element.ID == id {
			return element, true
		}
	}
	return Element{}, false
}
