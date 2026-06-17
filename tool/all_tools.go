package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
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
			if p.Pattern==""&&p.Term==""{return "",fmt.Errorf("需要 pattern 或 term")}
var matches []string
pattern:=p.Pattern
if pattern==""{pattern="."}
filepath.Walk(pattern,func(fp string,fi os.FileInfo,err error)error{
if err!=nil{return nil}
if p.Term!=""&&strings.Contains(fi.Name(),p.Term){matches=append(matches,fp)}
return nil
})
if len(matches)==0{return "无匹配",nil}
return fmt.Sprintf("匹配 %d 个文件：\n%s",len(matches),strings.Join(matches,"\n")),nil
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
			matches,err:=filepath.Glob(p.Pattern)
if err!=nil{return "",fmt.Errorf("glob: %v",err)}
if len(matches)==0{return "无匹配",nil}
return fmt.Sprintf("匹配 %d 个文件：\n%s",len(matches),strings.Join(matches,"\n")),nil
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
			var matches []string
if p.Path==""{p.Path="."}
filepath.Walk(p.Path,func(fp string,fi os.FileInfo,err error)error{
if err!=nil||fi.IsDir()||strings.HasPrefix(fi.Name(),"."){return nil}
data,err:=os.ReadFile(fp)
if err!=nil{return nil}
lines:=strings.Split(string(data),"\n")
for i,line:=range lines{
if strings.Contains(line,p.Pattern){
matches=append(matches,fmt.Sprintf("%s:%d: %s",fp,i+1,strings.TrimSpace(line)))
}
}
return nil
})
if len(matches)==0{return "无匹配",nil}
return fmt.Sprintf("匹配 %d 处：\n%s",len(matches),strings.Join(matches,"\n")),nil
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
			var matches []string
searchDir:=p.Path
if searchDir==""{searchDir="."}
filepath.Walk(searchDir,func(fp string,fi os.FileInfo,err error)error{
if err!=nil||fi.IsDir()||!strings.HasSuffix(fp,".go"){return nil}
data,err:=os.ReadFile(fp)
if err!=nil{return nil}
lines:=strings.Split(string(data),"\n")
for i,line:=range lines{
if strings.Contains(line,p.Symbol){
matches=append(matches,fmt.Sprintf("%s:%d: %s",fp,i+1,strings.TrimSpace(line)))
}
}
return nil
})
if len(matches)==0{return fmt.Sprintf("未找到符号 %s",p.Symbol),nil}
return fmt.Sprintf("找到 %d 处 %s：\n%s",len(matches),p.Symbol,strings.Join(matches,"\n")),nil
		},
	}
}


func stripHTML(s string) string {
s=regexp.MustCompile("(?is)<(?:script|style)[^>]*>.*?</(?:script|style)>").ReplaceAllString(s,"")
s=regexp.MustCompile("(?is)<!--.*?-->").ReplaceAllString(s,"")
s=regexp.MustCompile("(?is)<[^>]+>").ReplaceAllString(s,"")
repl:=strings.NewReplacer("&amp;","&","&lt;","<","&gt;",">","&quot;","\"","&#39;","'","&nbsp;"," ")
s=repl.Replace(s)
return regexp.MustCompile("\n[ \\t]*\\n([ \\t]*\\n)+").ReplaceAllString(s,"\n\n")
}
func NewWebFetchTool() Tool {
	return Tool{Name:"web_fetch",Description:"HTTP GET 抓取 URL",ReadOnly:true,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"url":map[string]any{"type":"string"}},"required":[]string{"url"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{URL string}
			json.Unmarshal(args,&p)
			if p.URL==""{return "",fmt.Errorf("url required")}
			client:=&http.Client{Timeout:15*time.Second}
req,err:=http.NewRequestWithContext(ctx,"GET",p.URL,nil)
if err!=nil{return "",fmt.Errorf("request: %v",err)}
req.Header.Set("User-Agent","QiuQiuPro/1.0")
resp,err:=client.Do(req)
if err!=nil{return "",fmt.Errorf("fetch: %v",err)}
defer resp.Body.Close()
body,err:=io.ReadAll(io.LimitReader(resp.Body,1<<20))
if err!=nil{return "",fmt.Errorf("read: %v",err)}
out:=string(body)
if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")),"text/html")||strings.Contains(out,"<!doctype")||strings.Contains(out,"<html"){
out=stripHTML(out)
}
if len(out)>16000{out=out[:16000]+"\n...(truncated)"}
return fmt.Sprintf("HTTP %s\n%s",resp.Status,strings.TrimSpace(out)),nil
		},
	}
}

func NewGitCommitTool() Tool {
	return Tool{Name:"git_commit",Description:"提交文件变更",ReadOnly:false,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"message":map[string]any{"type":"string"}},"required":[]string{"message"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Message string}
			json.Unmarshal(args,&p)
			cmd:=exec.CommandContext(ctx,"git","add","-A")
if out,err:=cmd.CombinedOutput();err!=nil{return fmt.Sprintf("git add failed: %s",out),err}
cmd=exec.CommandContext(ctx,"git","commit","-m",p.Message)
out,err:=cmd.CombinedOutput()
if err!=nil{return fmt.Sprintf("git commit failed: %s",out),err}
return strings.TrimSpace(string(out)),nil
		},
	}
}

func NewRunShellTool() Tool {
	return Tool{Name:"bash",Description:"执行 Shell 命令（PowerShell），返回 stdout+stderr。最大输出 32KB，超时 60s。",ReadOnly:false,
		Parameters: map[string]any{"type":"object","properties":map[string]any{"command":map[string]any{"type":"string","description":"要执行的命令"}},"required":[]string{"command"}},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{Command string}
			json.Unmarshal(args,&p)
			if p.Command==""{return "",fmt.Errorf("command required")}
			var cmd *exec.Cmd
if runtime.GOOS=="windows"{
	cmd=exec.CommandContext(ctx,"C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe","-NoProfile","-Command",p.Command)
}else{
	cmd=exec.CommandContext(ctx,"/bin/sh","-c",p.Command)
}
			out,err:=cmd.CombinedOutput()
			if err!=nil{
				outStr:=strings.TrimSpace(string(out))
				if outStr!=""{return outStr,err}
				return "",fmt.Errorf("command failed: %v",err)
			}
			output:=string(out)
			if len(output)>32000{output=output[:32000]+"\n...(截断)"}
			return strings.TrimSpace(output),nil
		},
	}
}
