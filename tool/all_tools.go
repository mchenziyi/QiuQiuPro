package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func NewReadFileTool() Tool {
	return Tool{Name:"read_file",Description:"读取指定文件的内容",ReadOnly:true,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"path":map[string]any{"type":"string","description":"文件路径"}},"required":[]string{"path"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Path string}
			json.Unmarshal(args,&p)
			data,err:=os.ReadFile(p.Path)
			if err!=nil{return "",fmt.Errorf("读取%s失败",p.Path)}
			return fmt.Sprintf("文件 %s（%d 字节）内容：\n%s",p.Path,len(data),string(data)),nil
		},
	}
}

func NewWriteFileTool() Tool {
	return Tool{Name:"write_file",Description:"创建或覆盖文件",ReadOnly:false,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"path":map[string]any{"type":"string","description":"文件路径"},"content":map[string]any{"type":"string","description":"内容"}},"required":[]string{"path","content"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Path,Content string}
			json.Unmarshal(args,&p)
			if err:=os.WriteFile(p.Path,[]byte(p.Content),0644);err!=nil{return "",fmt.Errorf("写入失败:%v",err)}
			return fmt.Sprintf("已写入 %s",p.Path),nil
		},
	}
}

func NewListDirectoryTool() Tool {
	return Tool{Name:"ls",Description:"列出目录下的文件和子目录",ReadOnly:true,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"path":map[string]any{"type":"string","description":"目录路径"}}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Path string}
			json.Unmarshal(args,&p)
			if p.Path==""{p.Path="."}
			entries,err:=os.ReadDir(p.Path)
			if err!=nil{return "",fmt.Errorf("读目录失败:%v",err)}
			var dirs,files []string
			for _,e:=range entries{if e.IsDir(){dirs=append(dirs,e.Name()+"/")}else{files=append(files,e.Name())}}
			var b strings.Builder
			if len(dirs)>0{b.WriteString("子目录："+strings.Join(dirs,"、")+"\n")}
			if len(files)>0{b.WriteString("文件：\n"+strings.Join(files,"\n"))}
			if b.Len()==0{b.WriteString("（空目录）")}
			return fmt.Sprintf("目录 %s：\n%s",p.Path,strings.TrimSpace(b.String())),nil
		},
	}
}

func NewEditFileTool() Tool {
	return Tool{Name:"edit_file",Description:"精确替换文件中的一段文本",ReadOnly:false,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"path":map[string]any{"type":"string"},"old_string":map[string]any{"type":"string"},"new_string":map[string]any{"type":"string"}},"required":[]string{"path","old_string","new_string"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Path,OldString,NewString string}
			json.Unmarshal(args,&p)
			b,err:=os.ReadFile(p.Path)
			if err!=nil{return "",fmt.Errorf("读取失败:%v",err)}
			content:=string(b)
			n:=strings.Count(content,p.OldString)
			if n==0{return "",fmt.Errorf("未找到 old_string")}
			if n>1{return "",fmt.Errorf("old_string 出现 %d 次",n)}
			if err:=os.WriteFile(p.Path,[]byte(strings.Replace(content,p.OldString,p.NewString,1)),0644);err!=nil{return "",fmt.Errorf("写入失败:%v",err)}
			return fmt.Sprintf("已编辑 %s",p.Path),nil
		},
	}
}

func NewSearchFilesTool() Tool {
	return Tool{Name:"search_files",Description:"按文件名或关键词搜索文件",ReadOnly:true,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"pattern":map[string]any{"type":"string"},"term":map[string]any{"type":"string"}}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Pattern,Term string}
			json.Unmarshal(args,&p)
			return fmt.Sprintf("搜索 %s 中 %q",p.Pattern,p.Term),nil
		},
	}
}

func NewGlobTool() Tool {
	return Tool{Name:"glob",Description:"按文件名模式搜索文件",ReadOnly:true,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"pattern":map[string]any{"type":"string"}},"required":[]string{"pattern"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Pattern string}
			json.Unmarshal(args,&p)
			if p.Pattern==""{return "",fmt.Errorf("pattern required")}
			return fmt.Sprintf("glob %q",p.Pattern),nil
		},
	}
}

func NewGrepTool() Tool {
	return Tool{Name:"grep",Description:"在文件内容中搜索关键词或正则",ReadOnly:true,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"pattern":map[string]any{"type":"string"},"path":map[string]any{"type":"string"}},"required":[]string{"pattern"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Pattern,Path string}
			json.Unmarshal(args,&p)
			if p.Pattern==""{return "",fmt.Errorf("pattern required")}
			return fmt.Sprintf("grep %q in %s",p.Pattern,p.Path),nil
		},
	}
}

func NewMultiEditTool() Tool {
	return Tool{Name:"multi_edit",Description:"批量编辑文件，原子性",ReadOnly:false,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"path":map[string]any{"type":"string"},"edits":map[string]any{"type":"array","items":map[string]any{"type":"object","properties":map[string]any{"old_string":map[string]any{"type":"string"},"new_string":map[string]any{"type":"string"},"replace_all":map[string]any{"type":"boolean"}},"required":[]string{"old_string","new_string"}}}},"required":[]string{"path","edits"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Path string;Edits []struct{OldString,NewString string;ReplaceAll bool}}
			json.Unmarshal(args,&p)
			b,err:=os.ReadFile(p.Path)
			if err!=nil{return "",fmt.Errorf("读取失败:%v",err)}
			content:=string(b)
			for i,step:=range p.Edits{
				if step.ReplaceAll{content=strings.ReplaceAll(content,step.OldString,step.NewString);continue}
				n:=strings.Count(content,step.OldString)
				if n==0{return "",fmt.Errorf("edit %d 未找到",i+1)}
				if n>1{return "",fmt.Errorf("edit %d 不唯一",i+1)}
				content=strings.Replace(content,step.OldString,step.NewString,1)
			}
			if err:=os.WriteFile(p.Path,[]byte(content),0644);err!=nil{return "",fmt.Errorf("写入失败:%v",err)}
			return fmt.Sprintf("已编辑 %s（%d 条）",p.Path,len(p.Edits)),nil
		},
	}
}

func NewDeleteRangeTool() Tool {
	return Tool{Name:"delete_range",Description:"按行锚点删除连续区域",ReadOnly:false,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"path":map[string]any{"type":"string"},"start_anchor":map[string]any{"type":"string"},"end_anchor":map[string]any{"type":"string"},"inclusive":map[string]any{"type":"boolean"}},"required":[]string{"path","start_anchor","end_anchor"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Path,StartAnchor,EndAnchor string;Inclusive*bool}
			json.Unmarshal(args,&p);inc:=true;if p.Inclusive!=nil{inc=*p.Inclusive}
			b,err:=os.ReadFile(p.Path)
			if err!=nil{return "",fmt.Errorf("读取失败:%v",err)}
			orig:=string(b);lines:=strings.Split(strings.ReplaceAll(orig,"\r",""),"\n")
			s:=findLine(lines,p.StartAnchor);if s<0{return "",fmt.Errorf("未找到 start_anchor")}
			e:=findLine(lines,p.EndAnchor);if e<0{return "",fmt.Errorf("未找到 end_anchor")}
			if s>e{return "",fmt.Errorf("anchor 顺序颠倒")}
			var keep []string
			if inc{keep=append(keep,lines[:s]...);keep=append(keep,lines[e+1:]...)}else{keep=append(keep,lines[:s+1]...);keep=append(keep,lines[e:]...)}
			newContent:=strings.Join(keep,"\n")
			if strings.HasSuffix(orig,"\n")&&!strings.HasSuffix(newContent,"\n"){newContent+="\n"}
			if err:=os.WriteFile(p.Path,[]byte(newContent),0644);err!=nil{return "",fmt.Errorf("写入失败:%v",err)}
			return fmt.Sprintf("已删除第 %d-%d 行",s+1,e+1),nil
		},
	}
}

func findLine(lines []string,target string)int{
	for i,l:=range lines{if l==target{return i}}
	return -1
}

func NewDeleteSymbolTool() Tool {
	return Tool{Name:"delete_symbol",Description:"按符号名从 Go 文件中精确删除",ReadOnly:false,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"path":map[string]any{"type":"string"},"name":map[string]any{"type":"string"},"kind":map[string]any{"type":"string"},"parent":map[string]any{"type":"string"}},"required":[]string{"path","name"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Path,Name,Kind,Parent string}
			json.Unmarshal(args,&p)
			return fmt.Sprintf("TODO: delete_symbol %s %s",p.Path,p.Name),nil
		},
	}
}

func NewTodoWriteTool() Tool {
	return Tool{Name:"todo_write",Description:"记录任务清单",ReadOnly:true,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"todos":map[string]any{"type":"array","items":map[string]any{"type":"object","properties":map[string]any{"content":map[string]any{"type":"string"},"status":map[string]any{"type":"string","enum":[]string{"pending","in_progress","completed"}},"activeForm":map[string]any{"type":"string"},"level":map[string]any{"type":"integer","enum":[]any{0,1}}}}}},"required":[]string{"todos"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Todos []struct{Content,Status,ActiveForm string;Level int}}
			json.Unmarshal(args,&p)
			var done,active,pending int
			for _,t:=range p.Todos{
				switch t.Status{
				case"completed":done++
				case"in_progress":active++
				default:pending++
				}
			}
			return fmt.Sprintf("Todos: %d done, %d active, %d pending",done,active,pending),nil
		},
	}
}

func NewCodeSearchTool() Tool {
	return Tool{Name:"code_search",Description:"按符号名搜索 Go 代码",ReadOnly:true,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"symbol":map[string]any{"type":"string"},"path":map[string]any{"type":"string"}},"required":[]string{"symbol"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Symbol,Path string}
			json.Unmarshal(args,&p)
			return fmt.Sprintf("搜索符号 %s",p.Symbol),nil
		},
	}
}

func NewWebFetchTool() Tool {
	return Tool{Name:"web_fetch",Description:"HTTP GET 抓取 URL",ReadOnly:true,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"url":map[string]any{"type":"string"}},"required":[]string{"url"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{URL string}
			json.Unmarshal(args,&p)
			if p.URL==""{return "",fmt.Errorf("url required")}
			return fmt.Sprintf("fetching %s...",p.URL),nil
		},
	}
}

func NewGitCommitTool() Tool {
	return Tool{Name:"git_commit",Description:"提交文件变更",ReadOnly:false,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"message":map[string]any{"type":"string"}},"required":[]string{"message"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Message string}
			json.Unmarshal(args,&p)
			return fmt.Sprintf("git commit: %s",p.Message),nil
		},
	}
}

func NewRunShellTool() Tool {
	return Tool{Name:"bash",Description:"执行 Shell 命令",ReadOnly:false,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"command":map[string]any{"type":"string"}},"required":[]string{"command"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Command string}
			json.Unmarshal(args,&p)
			return fmt.Sprintf("running: %s",p.Command),nil
		},
	}
}
