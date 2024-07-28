///////////////////////////////////////////////////////////////////////////////////////////////////
//                                                                                               //
//                                                                                               //
//         oooooo   oooooo     oooo           oooooo   oooooo     oooo         .o8               //
//          `888.    `888.     .8'             `888.    `888.     .8'         "888               //
//           `888.   .8888.   .8' oooo    ooo   `888.   .8888.   .8' .ooooo.   888oooo.          //
//            `888  .8'`888. .8'   `88.  .8'     `888  .8'`888. .8' d88' `88b  d88' `88b         //
//             `888.8'  `888.8'     `88..8'       `888.8'  `888.8'  888ooo888  888   888         //
//              `888'    `888'       `888'         `888'    `888'   888    .o  888   888         //
//               `8'      `8'         .8'           `8'      `8'    `Y8bod8P'  `Y8bod8P'         //
//                                .o..P'                                                         //
//                                `Y8P'                                                          //
//                                                                                               //
//                                                                                               //
//                              Copyright (C) 2024  Wyatt Sheffield                              //
//                                                                                               //
//                 This program is free software: you can redistribute it and/or                 //
//                 modify it under the terms of the GNU General Public License as                //
//                 published by the Free Software Foundation, either version 3 of                //
//                      the License, or (at your option) any later version.                      //
//                                                                                               //
//                This program is distributed in the hope that it will be useful,                //
//                 but WITHOUT ANY WARRANTY; without even the implied warranty of                //
//                 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the                 //
//                          GNU General Public License for more details.                         //
//                                                                                               //
//                   You should have received a copy of the GNU General Public                   //
//                         License along with this program.  If not, see                         //
//                                <https://www.gnu.org/licenses/>.                               //
//                                                                                               //
//                                                                                               //
///////////////////////////////////////////////////////////////////////////////////////////////////

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
