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

import (
	"log"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	gmutil "github.com/yuin/goldmark/util"

	"wyweb.site/util"
)

type linkRewriteTransformer struct {
	subdir string
}

func (r linkRewriteTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && (n.Kind() == ast.KindLink || n.Kind() == ast.KindImage) {
			var url *[]byte
			switch n.Kind() {
			case ast.KindLink:
				url = &n.(*ast.Link).Destination
			case ast.KindImage:
				url = &n.(*ast.Image).Destination
			}
			temp, err := util.RewriteURLPath(string(*url), r.subdir)
			if err != nil {
				log.Printf("Error transforming URL '%s' : %s\n", string(*url), err.Error())
			}
			*url = []byte(temp)
		}
		return ast.WalkContinue, nil
	})
}

type linkRewrite struct {
	subdir string
}

func (e *linkRewrite) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithASTTransformers(
			gmutil.Prioritized(linkRewriteTransformer{e.subdir}, priorityLinkRewriteTransformer),
		),
	)
}

func LinkRewrite(subdir string) goldmark.Extender {
	return &linkRewrite{subdir}
}
