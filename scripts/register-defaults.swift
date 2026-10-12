import Foundation
import CoreServices
import AppKit

let app = URL(fileURLWithPath: CommandLine.arguments[1]).standardizedFileURL
let bundleID = "dev.zipgallery.app"
let types: [(String,String)] = [
 ("zip","public.zip-archive"), ("cbz","cx.c3.cbz-archive"),
 ("7z","org.7-zip.7-zip-archive"), ("rar","com.rarlab.rar-archive"),
 ("tar","public.tar-archive"), ("gz","org.gnu.gnu-zip-archive"),
 ("bz2","public.bzip2-archive"), ("xz","org.tukaani.xz-archive"),
 ("tgz","org.gnu.gnu-zip-tar-archive")
]
var previous: [String:String] = [:]
for (_,uti) in types {
 previous[uti] = LSCopyDefaultRoleHandlerForContentType(uti as CFString,.all)?.takeRetainedValue() as String? ?? ""
}
let backup=URL(fileURLWithPath:CommandLine.arguments[2])
if !FileManager.default.fileExists(atPath:backup.path) {
 try JSONSerialization.data(withJSONObject:previous,options:[.prettyPrinted,.sortedKeys]).write(to:backup,options:.atomic)
}
let registered=LSRegisterURL(app as CFURL,true)
guard registered==noErr else {fatalError("Registration failed: \(registered)")}
for (ext,uti) in types {
 let result=LSSetDefaultRoleHandlerForContentType(uti as CFString,.all,bundleID as CFString)
 guard result==noErr else {fputs("Default association failed: \(ext), \(result)\n",stderr);exit(1)}
 print("\(ext): default requested")
}
if let resolved=NSWorkspace.shared.urlForApplication(withBundleIdentifier:bundleID){print("Application: \(resolved.path)")}
