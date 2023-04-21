package pkg

import (
	"fmt"
	"github.com/ahmetb/go-linq/v3"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

var variableAttributePriorities = map[string]int{
	"type":        0,
	"default":     1,
	"description": 2,
	"nullable":    3,
	"sensitive":   4,
}

var newLineToken = &hclwrite.Token{
	Type:  hclsyntax.TokenNewline,
	Bytes: []byte("\n"),
}

type VariablesFile struct {
	File *HclFile
}

func BuildVariablesFile(f *HclFile) *VariablesFile {
	return &VariablesFile{
		File: f,
	}
}

func (f *VariablesFile) AutoFix() {
	variableBlocks := make([]*VariableBlock, 0)
	for i := 0; i < len(f.File.WriteFile.Body().Blocks()); i++ {
		b := BuildVariableBlock(f.File.File, f.File.GetBlock(i))
		b.AutoFix()
		variableBlocks = append(variableBlocks, b)
	}
	linq.From(variableBlocks).OrderBy(func(i interface{}) interface{} {
		variableBlock := i.(*VariableBlock)
		name := variableBlock.Block.Labels[0]
		isRequired := isRequiredVariableBlock(variableBlock.Block.Block)
		prefix := "0"
		if !isRequired {
			prefix = "1"
		}
		return fmt.Sprintf("%s_%s", prefix, name)
	}).ToSlice(&variableBlocks)

	f.File.WriteFile.Body().Clear()

	for i, variableBlock := range variableBlocks {
		if i != 0 {
			f.File.appendNewline()
		}
		f.File.appendBlock(variableBlock.Block)
		if !endWithNewLine(variableBlock.Block.WriteBlock) {
			f.File.appendNewline()
		}
	}
}

type VariableBlock struct {
	Block      *HclBlock
	Attributes Args
}

func BuildVariableBlock(f *hcl.File, b *HclBlock) *VariableBlock {
	r := &VariableBlock{
		Block: b,
	}
	for _, attribute := range attributesByLines(b.Attributes()) {
		r.Attributes = append(r.Attributes, buildAttrArg(attribute, f))
	}
	return r
}

func (b *VariableBlock) AutoFix() {
	b.sortArguments()
	b.removeUnnecessaryNullable()
	b.removeUnnecessarySensitive()
	b.write()
}

func (b *VariableBlock) sortArguments() {
	linq.From(b.Attributes).OrderBy(func(i interface{}) interface{} {
		attr := i.(*Arg)
		return variableAttributePriorities[attr.Name]
	}).ToSlice(&b.Attributes)
}

func (b *VariableBlock) write() {
	attributes := b.Block.WriteBlock.Body().Attributes()
	blocks := b.Block.WriteBlock.Body().Blocks()
	b.Block.Clear()
	b.Block.appendNewline()
	b.Block.writeArgs(b.Attributes, attributes)
	if len(blocks) > 0 {
		validationBlock := blocks[0]
		b.Block.appendNewline()
		b.Block.appendBlock(validationBlock)
	}
}

func (b *VariableBlock) removeUnnecessaryNullable() {
	for i := 0; i < len(b.Attributes); i++ {
		attr := b.Attributes[i]
		if attr.Name != "nullable" {
			continue
		}
		literal, ok := attr.Attribute.Expr.(*hclsyntax.LiteralValueExpr)
		if ok && literal.Val.True() {
			b.Attributes = removeIndex(b.Attributes, i)
		}
		return
	}
}

func (b *VariableBlock) removeUnnecessarySensitive() {
	for i := 0; i < len(b.Attributes); i++ {
		attr := b.Attributes[i]
		if attr.Name != "sensitive" {
			continue
		}
		literal, ok := attr.Attribute.Expr.(*hclsyntax.LiteralValueExpr)
		if ok && literal.Val.False() {
			b.Attributes = removeIndex(b.Attributes, i)
		}
		return
	}
}

func isRequiredVariableBlock(b *hclsyntax.Block) bool {
	nullable, ok := b.Body.Attributes["nullable"]
	if !ok {
		return false
	}
	expr, ok := nullable.Expr.(*hclsyntax.LiteralValueExpr)
	return ok && expr.Val.False()
}
