// SPDX-License-Identifier: Apache-2.0
// Optional read-only acceptance fixture, never linked into the product.
// Reads pointer position and clipboard change count; never reads clipboard contents.
import AppKit
import Foundation
let p = NSEvent.mouseLocation
let observation: [String: Any] = ["pointerX": p.x, "pointerY": p.y,
                                "clipboardChangeCount": NSPasteboard.general.changeCount]
let data = try JSONSerialization.data(withJSONObject: observation, options: [.sortedKeys])
print(String(data: data, encoding: .utf8)!)
