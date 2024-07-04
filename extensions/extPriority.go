package extensions

/*
Default Block Parsers
=====================
SetextHeadingParser     100
ThematicBreakParser     200
ListParser              300
ListItemParser          400
CodeBlockParser         500
ATXHeadingParser        600
FencedCodeBlockParser   700
BlockquoteParser        800
HTMLBlockParser         900
ParagraphParser         1000

Default Inline Parsers
======================
CodeSpanParser   100
LinkParser       200
AutoLinkParser   300
RawHTMLParser    400
EmphasisParser   500

extensions
==========
footnoteID                   100

DefinitionListHTMLRenderer   500
FootnoteHTMLRenderer         500
StrikethroughHTMLRenderer    500
TableHTMLRenderer            500
TaskCheckBoxHTMLRenderer     500

TaskCheckBoxParser           0
DefinitionListParser         101
FootnoteParser               101
DefinitionDescriptionParser  102
StrikethroughParser          500
FootnoteBlockParser          999
LinkifyParser                999
TypographerParser            9999

defaultTableASTTransformer   0
tableStyleTransformer        0
TableParagraphTransformer    200
FootnoteASTTransformer       999
*/

const (
	priorityAlertParser            = 150 //Must be before links
	priorityAlertRenderer          = 1000
	priorityAlertTransformer       = 1000
	priorityAttribListParser       = 2000
	priorityAttribListTransformer  = 1000
	priorityLinkRewriteTransformer = 0
	priorityMediaHTMLRenderer      = 10000
	priorityMediaTransformer       = 9000
)
