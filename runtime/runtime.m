#import <UIKit/UIKit.h>
#import <Foundation/Foundation.h>
#import <objc/runtime.h>
#import <objc/message.h>
#include <dlfcn.h>
#include <stdint.h>
#include <stdbool.h>
#include <string.h>
#include <limits.h>
#include <float.h>
#include <math.h>
#include <stdlib.h>
#include <unistd.h>

static void gh_main(void);
static void gh_init(void);
static UIView *gh_root;

static NSString *gh_string(const char *bytes, size_t length) {
    return [[NSString alloc] initWithBytes:bytes length:length encoding:NSUTF8StringEncoding];
}

static NSString *gh_concat(NSString *left, NSString *right) {
    return [(left ?: @"") stringByAppendingString:(right ?: @"")];
}

static int gh_compare(NSString *left, NSString *right) {
    NSData *a = [(left ?: @"") dataUsingEncoding:NSUTF8StringEncoding];
    NSData *b = [(right ?: @"") dataUsingEncoding:NSUTF8StringEncoding];
    size_t size = a.length < b.length ? a.length : b.length;
    int order = size ? memcmp(a.bytes, b.bytes, size) : 0;
    return order ? order : (a.length > b.length) - (a.length < b.length);
}

static int32_t gh_len(NSString *text) {
    return (int32_t)[text lengthOfBytesUsingEncoding:NSUTF8StringEncoding];
}

static void gh_panic(NSString *text) {
    NSLog(@"[gohome] panic: %@", text);
    abort();
}

static int64_t gh_div(int64_t a, int64_t b) {
    if (!b) gh_panic(@"integer divide by zero");
    if (a == INT64_MIN && b == -1) return INT64_MIN;
    return a / b;
}

static int64_t gh_mod(int64_t a, int64_t b) {
    if (!b) gh_panic(@"integer divide by zero");
    if (a == INT64_MIN && b == -1) return 0;
    return a % b;
}

static uint64_t gh_udiv(uint64_t a, uint64_t b) {
    if (!b) gh_panic(@"integer divide by zero");
    return a / b;
}

static uint64_t gh_umod(uint64_t a, uint64_t b) {
    if (!b) gh_panic(@"integer divide by zero");
    return a % b;
}

static uint64_t gh_shift(uint64_t value, uint64_t count, int width, int right, int sign) {
    uint64_t mask = width == 64 ? UINT64_MAX : ((1ULL << width) - 1);
    value &= mask;
    bool negative = sign && (value & (1ULL << (width - 1)));
    if (count >= width) return right && negative ? mask : 0;
    if (!right) return (value << count) & mask;
    if (negative && count) return (value >> count) | (mask ^ (mask >> count));
    return value >> count;
}

static void gh_log(NSString *text) {
    NSLog(@"[gohome] %@", text);
}

static NSString *gh_text_int(int64_t value) { return [NSString stringWithFormat:@"%lld", value]; }
static NSString *gh_text_uint(uint64_t value) { return [NSString stringWithFormat:@"%llu", value]; }
static NSString *gh_text_float(double value) { return [NSString stringWithFormat:@"%.17g", value]; }
static NSString *gh_text_bool(bool value) { return value ? @"true" : @"false"; }
static NSString *gh_text_object(id value) { return [value description] ?: @"<nil>"; }

static void gh_alert(NSString *title, NSString *text) {
    if (![NSThread isMainThread]) {
        dispatch_async(dispatch_get_main_queue(), ^{ gh_alert(title, text); });
        return;
    }
    [[[UIAlertView alloc] initWithTitle:title message:text delegate:nil cancelButtonTitle:@"OK" otherButtonTitles:nil] show];
}

static id gh_class(NSString *name) {
    return NSClassFromString(name);
}

static id gh_new(NSString *name) {
    Class cls = NSClassFromString(name);
    if (!cls) { gh_log(gh_concat(@"class missing: ", name)); return nil; }
    return [[cls alloc] init];
}

static NSString *gh_encoding(const char *type) {
    while (strchr("rnNoORV", *type) && *type) ++type;
    if (*type == '@') return @"@";
    if (*type == '#') return @"@";
    return [NSString stringWithFormat:@"%c", *type];
}

static bool gh_signature(Method method, const char *result, const char *arguments) {
    if (!method) return false;
    char *returnType = method_copyReturnType(method);
    bool valid = [gh_encoding(returnType) isEqualToString:gh_encoding(result)];
    free(returnType);
    unsigned count = method_getNumberOfArguments(method);
    valid = valid && count == strlen(arguments) + 2;
    for (unsigned i = 2; valid && i < count; ++i) {
        char *type = method_copyArgumentType(method, i);
        char expected[] = {arguments[i - 2], 0};
        valid = [gh_encoding(type) isEqualToString:gh_encoding(expected)];
        free(type);
    }
    return valid;
}

static bool gh_message(id receiver, SEL selector, const char *result, const char *arguments) {
    if (!receiver) return true;
    Method method = class_getInstanceMethod(object_getClass(receiver), selector);
    if (gh_signature(method, result, arguments)) return true;
    gh_log(gh_concat(@"message missing or ABI mismatch: ", NSStringFromSelector(selector)));
    return false;
}

static bool gh_hook(NSString *className, NSString *selectorName, IMP replacement, IMP *original, const char *result, const char *arguments, bool meta) {
    Class cls = NSClassFromString(className);
    if (meta && cls) cls = object_getClass(cls);
    SEL selector = NSSelectorFromString(selectorName);
    Method method = cls ? class_getInstanceMethod(cls, selector) : NULL;
    if (!method) { gh_log(gh_concat(@"hook method missing: ", gh_concat(className, gh_concat(@" ", selectorName)))); return false; }
    if (!gh_signature(method, result, arguments)) { gh_log(gh_concat(@"hook ABI mismatch: ", gh_concat(className, gh_concat(@" ", selectorName)))); return false; }
    void *handle = dlopen("/Library/Frameworks/CydiaSubstrate.framework/CydiaSubstrate", RTLD_LAZY);
    if (!handle) handle = dlopen("/usr/lib/libsubstrate.dylib", RTLD_LAZY);
    void (*hook)(Class, SEL, IMP, IMP *) = dlsym(handle ?: RTLD_DEFAULT, "MSHookMessageEx");
    if (!hook) { gh_log(@"Cydia Substrate is unavailable"); return false; }
    hook(cls, selector, replacement, original);
    return true;
}

static id gh_root_view(void) { return gh_root; }
static void gh_set_text(id object, NSString *text) { [(UILabel *)object setText:text]; }
static void gh_set_frame(id object, double x, double y, double width, double height) { [(UIView *)object setFrame:CGRectMake(x, y, width, height)]; }
static void gh_add_subview(id parent, id child) { [(UIView *)parent addSubview:child]; }

// gh_layout places a view and derives an autoresizing mask from the geometry, so
// that layouts written for a 320 point wide iPhone stretch and recentre on an
// iPad and survive rotation. A negative width or height mirrors the opposite
// margin, so -1 at x 20 means a symmetric 20 point inset on both sides, and at
// x 0 a full bleed view that stretches with the parent.
static UIViewAutoresizing gh_layout(UIView *view, double x, double y, double width, double height) {
    CGRect parent = gh_root ? gh_root.bounds : CGRectMake(0, 0, 320, 480);
    UIViewAutoresizing mask = UIViewAutoresizingNone;
    if (width < 0) {
        width = parent.size.width - x * 2;
        mask |= x ? UIViewAutoresizingFlexibleLeftMargin | UIViewAutoresizingFlexibleRightMargin : UIViewAutoresizingFlexibleWidth;
    } else if (fabs(x * 2 + width - parent.size.width) < 0.5) {
        mask |= UIViewAutoresizingFlexibleLeftMargin | UIViewAutoresizingFlexibleRightMargin;
    } else if (fabs(width - parent.size.width) < 0.5) {
        mask |= UIViewAutoresizingFlexibleWidth;
    } else if (fabs(x + width - parent.size.width) < 0.5) {
        mask |= UIViewAutoresizingFlexibleLeftMargin;
    }
    if (height < 0) {
        height = parent.size.height - y * 2;
        mask |= y ? UIViewAutoresizingFlexibleTopMargin | UIViewAutoresizingFlexibleBottomMargin : UIViewAutoresizingFlexibleHeight;
    } else if (fabs(y * 2 + height - parent.size.height) < 0.5) {
        mask |= UIViewAutoresizingFlexibleTopMargin | UIViewAutoresizingFlexibleBottomMargin;
    } else if (fabs(height - parent.size.height) < 0.5) {
        mask |= UIViewAutoresizingFlexibleHeight;
    } else if (fabs(y + height - parent.size.height) < 0.5) {
        mask |= UIViewAutoresizingFlexibleBottomMargin;
    }
    view.frame = CGRectMake(x, y, width, height);
    view.autoresizingMask = mask;
    return mask;
}

static id gh_label(NSString *text, double x, double y, double width, double height) {
    UILabel *label = [[UILabel alloc] initWithFrame:CGRectMake(x, y, width, height)];
    label.text = text;
    label.backgroundColor = [UIColor clearColor];
    [gh_root addSubview:label];
    gh_layout(label, x, y, width, height);
    return label;
}

typedef struct {
	NSString *title;
	NSString *icon;
	void (*build)(id);
	bool built;
} GHTab;

static GHTab gh_tabs[8];
static int gh_tab_count;

// gh_shape_image draws a tab bar icon with Core Graphics at the screen scale,
// which is the classic 30 by 30 point icon, 60 by 60 pixels on a retina screen.
static UIImage *gh_shape_image(NSString *shape) {
    CGFloat scale = [[UIScreen mainScreen] scale];
    if (scale <= 1) scale = 1;
    CGFloat side = 30.0 * scale;
    UIGraphicsBeginImageContextWithOptions(CGSizeMake(side, side), NO, scale);
    CGContextRef context = UIGraphicsGetCurrentContext();
    CGRect box = CGRectMake(side * 0.18, side * 0.18, side * 0.64, side * 0.64);
    CGContextSetLineWidth(context, side * 0.07);
    CGContextSetLineJoin(context, kCGLineJoinRound);
    CGContextSetRGBStrokeColor(context, 0.42, 0.48, 0.56, 1.0);
    if ([shape isEqualToString:@"circle"]) {
        CGContextAddEllipseInRect(context, box);
    } else if ([shape isEqualToString:@"triangle"]) {
        CGContextMoveToPoint(context, CGRectGetMidX(box), CGRectGetMinY(box));
        CGContextAddLineToPoint(context, CGRectGetMaxX(box), CGRectGetMaxY(box));
        CGContextAddLineToPoint(context, CGRectGetMinX(box), CGRectGetMaxY(box));
        CGContextClosePath(context);
    } else {
        CGContextAddRect(context, box);
    }
    CGContextStrokePath(context);
    CGImageRef drawn = UIGraphicsGetImageFromCurrentImageContext().CGImage;
    UIGraphicsEndImageContext();
    return [UIImage imageWithCGImage:drawn scale:scale orientation:UIImageOrientationUp];
}

static void gh_tab(NSString *title, NSString *icon, void (*build)(id)) {
    if (gh_tab_count >= 8) {
        gh_log(@"an app can register at most 8 tabs");
        return;
    }
    gh_tabs[gh_tab_count].title = title ?: @"";
    gh_tabs[gh_tab_count].icon = icon ?: @"";
    gh_tabs[gh_tab_count].build = build;
    gh_tabs[gh_tab_count].built = false;
    gh_tab_count++;
}

@interface GHAction : NSObject {
    void (*_callback)(void);
}
- (id)initWithCallback:(void (*)(void))callback;
- (void)invoke:(id)sender;
@end

@implementation GHAction
- (id)initWithCallback:(void (*)(void))callback {
    self = [super init];
    if (self) _callback = callback;
    return self;
}
- (void)invoke:(id)sender { if (_callback) _callback(); }
@end

static id gh_button(NSString *text, double x, double y, double width, double height, void (*callback)(void)) {
    UIButton *button = [UIButton buttonWithType:UIButtonTypeRoundedRect];
    button.frame = CGRectMake(x, y, width, height);
    [button setTitle:text forState:UIControlStateNormal];
    GHAction *action = [[GHAction alloc] initWithCallback:callback];
    objc_setAssociatedObject(button, @selector(invoke:), action, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    [button addTarget:action action:@selector(invoke:) forControlEvents:UIControlEventTouchUpInside];
    [gh_root addSubview:button];
    gh_layout(button, x, y, width, height);
    return button;
}

// gh_show makes a tab's page the current root view and runs its builder once,
// so ios.Label and ios.Button land on the visible tab. Tabs are only known
// after main has run, so an app that registers tabs must draw inside its
// builders rather than in main.
static void gh_show(UINavigationController *nav, GHTab *entry) {
    if (!nav || !entry) return;
    UIViewController *page = [nav.viewControllers objectAtIndex:0];
    gh_root = page.view;
    gh_root.backgroundColor = [UIColor whiteColor];
    if (!entry->built) {
        entry->built = true;
        if (entry->build) entry->build(gh_root);
    }
}

@interface GHTabs : UITabBarController <UITabBarControllerDelegate>
@end

@implementation GHTabs
- (void)tabBarController:(UITabBarController *)controller didSelectViewController:(UIViewController *)selected {
    NSUInteger index = [controller.viewControllers indexOfObject:selected];
    if (index == NSNotFound) return;
    gh_show((UINavigationController *)selected, &gh_tabs[index]);
}
@end

static UITabBarController *gh_build_tabs(void) {
    GHTabs *tabs = [[GHTabs alloc] init];
    NSMutableArray *controllers = [NSMutableArray array];
    for (int index = 0; index < gh_tab_count; index++) {
        UIViewController *page = [[UIViewController alloc] init];
        page.title = gh_tabs[index].title;
        UINavigationController *nav = [[UINavigationController alloc] initWithRootViewController:page];
        nav.tabBarItem = [[UITabBarItem alloc] initWithTitle:gh_tabs[index].title image:nil tag:index];
        if (gh_tabs[index].icon.length) nav.tabBarItem.image = gh_shape_image(gh_tabs[index].icon);
        [controllers addObject:nav];
    }
    [tabs setViewControllers:controllers animated:NO];
    tabs.delegate = tabs;
    return tabs;
}

static bool gh_text(id value) { return [value isKindOfClass:[NSString class]]; }

static bool gh_number(id value) { return [value isKindOfClass:[NSNumber class]]; }

static NSString *gh_render(id value) {
    if (!value) return @"<nil>";
    if (gh_text(value)) return value;
    return [value description] ?: @"<nil>";
}

static int32_t gh_width(unichar c) {
    if (c >= 0xD800 && c <= 0xDFFF) return c >= 0xDC00 ? 1 : 3;
    return c < 0x80 ? 1 : (c < 0x800 ? 2 : 3);
}

static int32_t gh_utf8_offset(NSString *text, NSUInteger units) {
    if (units > text.length) units = text.length;
    int32_t bytes = 0;
    for (NSUInteger i = 0; i < units; i++) bytes += gh_width([text characterAtIndex:i]);
    return bytes;
}

static NSUInteger gh_unit_offset(NSString *text, int32_t bytes) {
    NSUInteger units = 0;
    int32_t walked = 0;
    while (units < text.length && walked < bytes) {
        walked += gh_width([text characterAtIndex:units]);
        units++;
    }
    return units;
}

static NSString *gh_rune_string(int32_t r) {
    unichar pair[2];
    if (r >= 0x10000 && r <= 0x10FFFF) {
        int32_t offset = r - 0x10000;
        pair[0] = (unichar)(0xD800 + (offset >> 10));
        pair[1] = (unichar)(0xDC00 + (offset & 0x3FF));
        return [NSString stringWithCharacters:pair length:2];
    }
    if (r < 0 || r > 0xFFFF) return nil;
    pair[0] = (unichar)r;
    return [NSString stringWithCharacters:pair length:1];
}

static uint8_t gh_byte(NSString *text, int32_t index) {
    NSData *data = [text dataUsingEncoding:NSUTF8StringEncoding];
    if (index < 0 || (NSUInteger)index >= data.length) gh_panic(@"index out of range");
    return ((const uint8_t *)data.bytes)[index];
}

static int32_t gh_rune(NSString *text, int32_t index, int32_t *next) {
    NSData *data = [text dataUsingEncoding:NSUTF8StringEncoding];
    if (index < 0 || (NSUInteger)index >= data.length) {
        if (next) *next = index + 1;
        return 0xFFFD;
    }
    const uint8_t *p = (const uint8_t *)data.bytes;
    uint8_t lead = p[index];
    int32_t size = lead < 0x80 ? 1 : (lead < 0xE0 ? 2 : (lead < 0xF0 ? 3 : 4));
    if (next) *next = index + size;
    uint32_t value = size == 1 ? lead : (lead & (0xFF >> (size + 1)));
    for (int32_t i = 1; i < size && (NSUInteger)(index + i) < data.length; i++) {
        value = (value << 6) | (p[index + i] & 0x3F);
    }
    return (int32_t)value;
}

static bool gh_has_prefix(NSString *s, NSString *prefix) {
    if (!prefix.length) return true;
    return prefix.length <= s.length && [s rangeOfString:prefix].location == 0;
}

static bool gh_has_suffix(NSString *s, NSString *suffix) {
    if (!suffix.length) return true;
    if (suffix.length > s.length) return false;
    return [s rangeOfString:suffix options:NSBackwardsSearch].location != NSNotFound;
}

static int32_t gh_index(NSString *s, NSString *substr) {
    if (!substr.length) return 0;
    NSRange range = [s rangeOfString:substr];
    return range.location == NSNotFound ? -1 : gh_utf8_offset(s, range.location);
}

static int32_t gh_last_index(NSString *s, NSString *substr) {
    if (!substr.length) return gh_len(s);
    NSRange range = [s rangeOfString:substr options:NSBackwardsSearch];
    return range.location == NSNotFound ? -1 : gh_utf8_offset(s, range.location);
}

static bool gh_contains_any(NSString *s, NSString *chars) {
    for (NSUInteger i = 0; i < chars.length; i++) {
        unichar c = [chars characterAtIndex:i];
        if (c >= 0xD800 && c <= 0xDBFF && i + 1 < chars.length) {
            unichar pair[2] = {c, [chars characterAtIndex:i + 1]};
            if ([s rangeOfString:[NSString stringWithCharacters:pair length:2]].location != NSNotFound) return true;
            i++;
            continue;
        }
        if ([s rangeOfString:[NSString stringWithCharacters:&c length:1]].location != NSNotFound) return true;
    }
    return false;
}

static int32_t gh_index_any(NSString *s, NSString *chars) {
    int32_t best = -1;
    for (NSUInteger i = 0; i < chars.length; i++) {
        unichar c = [chars characterAtIndex:i];
        NSString *needle = [NSString stringWithCharacters:&c length:1];
        NSRange range = [s rangeOfString:needle];
        if (range.location == NSNotFound) continue;
        if (best < 0 || range.location < (NSUInteger)best) best = gh_utf8_offset(s, range.location);
    }
    return best;
}

static int32_t gh_count(NSString *s, NSString *substr) {
    if (!substr.length) return gh_len(s) + 1;
    int32_t total = 0;
    NSUInteger at = 0;
    while (at < s.length) {
        NSRange range = [s rangeOfString:substr options:0 range:NSMakeRange(at, s.length - at)];
        if (range.location == NSNotFound) break;
        total++;
        at = range.location + range.length;
    }
    return total;
}

static int32_t gh_strings_compare(NSString *a, NSString *b) { return gh_compare(a, b); }

static bool gh_equal_fold(NSString *a, NSString *b) {
    return [a caseInsensitiveCompare:b] == NSOrderedSame;
}

static NSString *gh_replace(NSString *s, NSString *old, NSString *new, int32_t n) {
    if (!old.length) return s;
    NSMutableString *out = [NSMutableString string];
    NSUInteger at = 0;
    int32_t done = 0;
    while (at <= s.length && (n < 0 || done < n)) {
        NSRange range = [s rangeOfString:old options:0 range:NSMakeRange(at, s.length - at)];
        if (range.location == NSNotFound) break;
        [out appendString:[s substringWithRange:NSMakeRange(at, range.location - at)]];
        [out appendString:new];
        at = range.location + range.length;
        done++;
    }
    [out appendString:[s substringFromIndex:at]];
    return out;
}

static NSString *gh_repeat(NSString *s, int32_t count) {
    if (count <= 0 || !s.length) return @"";
    NSMutableString *out = [NSMutableString string];
    for (int32_t i = 0; i < count; i++) [out appendString:s];
    return out;
}

static NSString *gh_trim(NSString *s, NSString *cutset) {
    NSUInteger start = 0, end = s.length;
    while (start < end) {
        unichar c = [s characterAtIndex:start];
        if ([cutset rangeOfString:[NSString stringWithCharacters:&c length:1]].location == NSNotFound) break;
        start++;
    }
    while (end > start) {
        unichar c = [s characterAtIndex:end - 1];
        if ([cutset rangeOfString:[NSString stringWithCharacters:&c length:1]].location == NSNotFound) break;
        end--;
    }
    return [s substringWithRange:NSMakeRange(start, end - start)];
}

static NSString *gh_trim_left(NSString *s, NSString *cutset) {
    NSUInteger start = 0;
    while (start < s.length) {
        unichar c = [s characterAtIndex:start];
        if ([cutset rangeOfString:[NSString stringWithCharacters:&c length:1]].location == NSNotFound) break;
        start++;
    }
    return [s substringFromIndex:start];
}

static NSString *gh_trim_right(NSString *s, NSString *cutset) {
    NSUInteger end = s.length;
    while (end > 0) {
        unichar c = [s characterAtIndex:end - 1];
        if ([cutset rangeOfString:[NSString stringWithCharacters:&c length:1]].location == NSNotFound) break;
        end--;
    }
    return [s substringToIndex:end];
}

static NSString *gh_trim_prefix(NSString *s, NSString *prefix) {
    return gh_has_prefix(s, prefix) ? [s substringFromIndex:prefix.length] : s;
}

static NSString *gh_trim_suffix(NSString *s, NSString *suffix) {
    if (!gh_has_suffix(s, suffix)) return s;
    return [s substringToIndex:s.length - suffix.length];
}

static NSString *gh_index_rune_string(int32_t r) { return gh_rune_string(r) ?: @""; }

static int32_t gh_index_rune(NSString *s, int32_t r) {
    NSString *needle = gh_index_rune_string(r);
    return needle.length ? gh_index(s, needle) : -1;
}

static NSString *gh_strconv_itoa(int64_t i) {
    return [NSString stringWithFormat:@"%lld", i];
}

static bool gh_contains(NSString *s, NSString *substr) {
    return !substr.length ? true : [s rangeOfString:substr].location != NSNotFound;
}

static bool gh_contains_rune(NSString *s, int32_t r) {
    NSString *needle = gh_index_rune_string(r);
    return needle.length ? gh_contains(s, needle) : false;
}

static NSString *gh_to_lower(NSString *s) { return [s lowercaseString]; }

static NSString *gh_to_upper(NSString *s) { return [s uppercaseString]; }

static NSString *gh_to_title(NSString *s) { return [s capitalizedString]; }

static NSString *gh_trim_space(NSString *s) {
    return [s stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
}

static NSString *gh_strconv_format_bool(bool b) { return b ? @"true" : @"false"; }

static NSString *gh_strconv_format_int(int64_t i, int32_t base) {
    if (base == 16) return [NSString stringWithFormat:@"%llx", (unsigned long long)i];
    if (base == 8) return [NSString stringWithFormat:@"%llo", (unsigned long long)i];
    if (base == 2) {
        if (!i) return @"0";
        unsigned long long value = (unsigned long long)i;
        NSMutableString *out = [NSMutableString string];
        char digits[72];
        int at = 0;
        while (value && at < 64) {
            digits[at++] = (char)('0' + (int)(value & 1));
            value >>= 1;
        }
        while (at) [out appendFormat:@"%c", digits[--at]];
        return out;
    }
    return [NSString stringWithFormat:@"%lld", i];
}

static double gh_strconv_parse_float(NSString *s, int32_t bitSize) {
    NSScanner *scanner = [NSScanner scannerWithString:s];
    double value = 0;
    if (![scanner scanDouble:&value] || ![scanner isAtEnd]) return 0;
    return bitSize == 32 ? (float)value : value;
}

static int gh_digit(unichar c, int32_t base) {
    int value = -1;
    if (c >= '0' && c <= '9') value = c - '0';
    else if (c >= 'a' && c <= 'z') value = c - 'a' + 10;
    else if (c >= 'A' && c <= 'Z') value = c - 'A' + 10;
    return value < (int)base ? value : -1;
}

static int64_t gh_strconv_parse_int(NSString *s, int32_t base, int64_t fallback) {
    NSString *text = [s stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
    NSUInteger at = 0;
    bool negative = false;
    if (at < text.length && ([text characterAtIndex:at] == '-' || [text characterAtIndex:at] == '+')) {
        negative = [text characterAtIndex:at] == '-';
        at++;
    }
    if (at + 1 < text.length && [text characterAtIndex:at] == '0') {
        unichar marker = [text characterAtIndex:at + 1];
        int32_t wanted = marker == 'x' || marker == 'X' ? 16 : (marker == 'o' || marker == 'O' ? 8 : (marker == 'b' || marker == 'B' ? 2 : 0));
        if (wanted && (base == 0 || base == wanted)) {
            base = wanted;
            at += 2;
        }
    }
    if (!base) base = 10;
    if (at >= text.length) return fallback;
    int64_t value = 0;
    for (; at < text.length; at++) {
        int digit = gh_digit([text characterAtIndex:at], base);
        if (digit < 0) return fallback;
        value = value * base + digit;
    }
    return negative ? -value : value;
}

static int32_t gh_strconv_atoi(NSString *s) { return (int32_t)gh_strconv_parse_int(s, 10, 0); }

static bool gh_strconv_parse_bool(NSString *s) {
    NSString *text = [s lowercaseString];
    return [text isEqualToString:@"1"] || [text isEqualToString:@"t"] || [text isEqualToString:@"true"];
}

static NSString *gh_strconv_format_float(double value, int32_t format, int32_t precision, int32_t bitSize) {
    if (bitSize == 32) value = (float)value;
    if (precision < 0) {
        switch (format) {
            case 'b': return [NSString stringWithFormat:@"%a", value];
            case 'e': case 'E': return [NSString stringWithFormat:@"%.*e", 6, value];
            default: return [NSString stringWithFormat:@"%.*g", 17, value];
        }
    }
    switch (format) {
        case 'b': return [NSString stringWithFormat:@"%.*e", precision, value];
        case 'e': return [NSString stringWithFormat:@"%.*e", precision, value];
        case 'E': {
            NSString *text = [NSString stringWithFormat:@"%.*e", precision, value];
            return [text stringByReplacingOccurrencesOfString:@"e" withString:@"E"];
        }
        case 'g': case 'G': {
            NSString *text = [NSString stringWithFormat:@"%.*g", precision, value];
            return format == 'G' ? [text stringByReplacingOccurrencesOfString:@"e" withString:@"E"] : text;
        }
        default: return [NSString stringWithFormat:@"%.*f", precision, value];
    }
}

static NSString *gh_strconv_unquote(NSString *s) {
    if (s.length < 2) return s;
    if ([s characterAtIndex:0] != '"' || [s characterAtIndex:s.length - 1] != '"') return s;
    NSString *body = [s substringWithRange:NSMakeRange(1, s.length - 2)];
    NSMutableString *out = [NSMutableString string];
    for (NSUInteger i = 0; i < body.length; i++) {
        unichar c = [body characterAtIndex:i];
        if (c == '\\' && i + 1 < body.length) {
            unichar next = [body characterAtIndex:i + 1];
            if (next == 'n') { [out appendString:@"\n"]; i++; continue; }
            if (next == 't') { [out appendString:@"\t"]; i++; continue; }
            if (next == 'r') { [out appendString:@"\r"]; i++; continue; }
            if (next == '"') { [out appendString:@"\""]; i++; continue; }
            if (next == '\\') { [out appendString:@"\\"]; i++; continue; }
        }
        [out appendFormat:@"%C", c];
    }
    return out;
}

static NSString *gh_quote(NSString *text) {
    NSMutableString *out = [NSMutableString stringWithString:@"\""];
    for (NSUInteger i = 0; i < text.length; i++) {
        unichar c = [text characterAtIndex:i];
        if (c == '"' || c == '\\') [out appendFormat:@"\\%C", c];
        else if (c < 0x20 || c == 0x7f) [out appendFormat:@"\\x%02x", c];
        else [out appendFormat:@"%C", c];
    }
    [out appendString:@"\""];
    return out;
}

static NSString *gh_strconv_quote(NSString *s) { return gh_quote(s); }


static NSString *gh_verb(NSString *spec, unichar verb, id arg, bool missing) {
    if (missing) return [NSString stringWithFormat:@"%%!%C(MISSING)", verb];
    if (verb == 't') {
        if (!gh_number(arg)) return [NSString stringWithFormat:@"%%!t(%C=%@)", verb, gh_render(arg)];
        return [arg boolValue] ? @"true" : @"false";
    }
    if (verb == 'v' || verb == 's') return gh_render(arg);
    if (verb == 'q') return gh_quote(gh_render(arg));
    if (verb == 'c') {
        unichar code = gh_number(arg) ? (unichar)[arg longLongValue] : '?';
        return gh_quote([NSString stringWithCharacters:&code length:1]);
    }
    if (verb == 'd' || verb == 'b' || verb == 'o' || verb == 'x' || verb == 'X') {
        if (!gh_number(arg)) return [NSString stringWithFormat:@"%%!%C(%C=%@)", verb, verb, gh_render(arg)];
        NSString *pattern = [NSString stringWithFormat:@"%@ll%C", [spec substringToIndex:spec.length - 1], verb];
        return [NSString stringWithFormat:pattern, (long long)[arg longLongValue]];
    }
    if (verb == 'e' || verb == 'E' || verb == 'f' || verb == 'F' || verb == 'g' || verb == 'G') {
        if (!gh_number(arg)) return [NSString stringWithFormat:@"%%!%C(%C=%@)", verb, verb, gh_render(arg)];
        return [NSString stringWithFormat:spec, [arg doubleValue]];
    }
    return [NSString stringWithFormat:@"%%!%C(%C=%@)", verb, verb, gh_render(arg)];
}

static NSString *gh_format(NSString *format, NSArray *args) {
    NSMutableString *out = [NSMutableString string];
    NSUInteger length = format.length, next = 0;
    for (NSUInteger i = 0; i < length; i++) {
        unichar c = [format characterAtIndex:i];
        if (c != '%') {
            [out appendFormat:@"%C", c];
            continue;
        }
        NSUInteger start = i;
        while (i + 1 < length && strchr("+-# 0.123456789", (int)[format characterAtIndex:i + 1])) i++;
        if (i + 1 >= length) {
            [out appendString:@"%!(NOVERB)"];
            break;
        }
        unichar verb = [format characterAtIndex:i + 1];
        if (verb == '%') {
            [out appendString:@"%"];
            i++;
            continue;
        }
        bool missing = next >= args.count;
        id arg = missing ? nil : [args objectAtIndex:next];
        if (!missing) next++;
        NSString *spec = [format substringWithRange:NSMakeRange(start, i + 2 - start)];
        [out appendString:gh_verb(spec, verb, arg, missing)];
        i++;
    }
    if (next < args.count) {
        [out appendString:@"%!(EXTRA "];
        for (NSUInteger i = next; i < args.count; i++) {
            if (i > next) [out appendString:@", "];
            [out appendString:gh_render([args objectAtIndex:i])];
        }
        [out appendString:@")"];
    }
    return out;
}

static NSString *gh_sprint(NSArray *args, bool spaces, bool newline) {
    NSMutableString *out = [NSMutableString string];
    id previous = nil;
    for (NSUInteger i = 0; i < args.count; i++) {
        id arg = [args objectAtIndex:i];
        if (i && (spaces || !(gh_text(arg) && gh_text(previous)))) [out appendString:@" "];
        [out appendString:gh_render(arg)];
        previous = arg;
    }
    if (newline) [out appendString:@"\n"];
    return out;
}

static void gh_fmt_print(NSArray *args) { gh_log(gh_sprint(args, false, false)); }

static void gh_fmt_println(NSArray *args) { gh_log(gh_sprint(args, true, true)); }

static void gh_fmt_printf(NSString *format, NSArray *args) { gh_log(gh_format(format, args)); }

static NSString *gh_fmt_sprint(NSArray *args) { return gh_sprint(args, false, false); }

static NSString *gh_fmt_sprintln(NSArray *args) { return gh_sprint(args, true, true); }

static NSString *gh_fmt_sprintf(NSString *format, NSArray *args) { return gh_format(format, args); }

static double gh_math_abs(double x) { return fabs(x); }
static double gh_math_acos(double x) { return acos(x); }
static double gh_math_acosh(double x) { return acosh(x); }
static double gh_math_asin(double x) { return asin(x); }
static double gh_math_asinh(double x) { return asinh(x); }
static double gh_math_atan(double x) { return atan(x); }
static double gh_math_atan2(double y, double x) { return atan2(y, x); }
static double gh_math_atanh(double x) { return atanh(x); }
static double gh_math_cbrt(double x) { return cbrt(x); }
static double gh_math_ceil(double x) { return ceil(x); }
static double gh_math_copysign(double f, double sign) { return copysign(f, sign); }
static double gh_math_cos(double x) { return cos(x); }
static double gh_math_cosh(double x) { return cosh(x); }
static double gh_math_exp(double x) { return exp(x); }
static double gh_math_floor(double x) { return floor(x); }
static double gh_math_hypot(double p, double q) { return hypot(p, q); }
static double gh_math_log(double x) { return log(x); }
static double gh_math_log10(double x) { return log10(x); }
static double gh_math_log2(double x) { return log2(x); }
static double gh_math_max(double x, double y) { return x > y ? x : y; }
static double gh_math_min(double x, double y) { return x < y ? x : y; }
static double gh_math_mod(double x, double y) { return fmod(x, y); }
static double gh_math_pow(double x, double y) { return pow(x, y); }
static double gh_math_round(double x) { return x >= 0 ? floor(x + 0.5) : ceil(x - 0.5); }
static double gh_math_sin(double x) { return sin(x); }
static double gh_math_sinh(double x) { return sinh(x); }
static double gh_math_sqrt(double x) { return sqrt(x); }
static double gh_math_tan(double x) { return tan(x); }
static double gh_math_tanh(double x) { return tanh(x); }
static double gh_math_trunc(double x) { return x >= 0 ? floor(x) : ceil(x); }

static double gh_math_inf(int32_t sign) { return sign >= 0 ? HUGE_VAL : -HUGE_VAL; }

static double gh_math_nan(void) { return __builtin_nan(""); }

static bool gh_math_is_nan(double f) { return f != f; }

static bool gh_math_is_inf(double f, int32_t sign) {
    if (f != f) return false;
    double magnitude = f < 0 ? -f : f;
    if (magnitude <= DBL_MAX) return false;
    return sign >= 0 ? f > 0 : f < 0;
}

static bool gh_math_signbit(double x) {
    if (x != x) return false;
    double magnitude = x < 0 ? -x : x;
    if (magnitude == 0) return false;
    return magnitude > DBL_MAX || x < 0;
}

static uint64_t gh_random_state = 0x2545f4914f6cdd1dULL;

static uint64_t gh_random(void) {
    uint64_t x = gh_random_state;
    x ^= x >> 12;
    x ^= x << 25;
    x ^= x >> 27;
    gh_random_state = x;
    return x * 0x2545f4914f6cdd1dULL;
}

static void gh_rand_seed(int64_t seed) {
    gh_random_state = (uint64_t)seed + 0x9e3779b97f4a7c15ULL;
    if (!gh_random_state) gh_random_state = 0x2545f4914f6cdd1dULL;
}

static int32_t gh_rand_int(void) { return (int32_t)(gh_random() >> 33); }

static int32_t gh_rand_int31(void) { return (int32_t)(gh_random() >> 33); }

static int32_t gh_rand_int31n(int32_t n) {
    if (n <= 0) gh_panic(@"invalid argument to Int31n");
    return (int32_t)(gh_random() % (uint64_t)n);
}

static int64_t gh_rand_int63(void) { return (int64_t)(gh_random() >> 1); }

static int64_t gh_rand_int63n(int64_t n) {
    if (n <= 0) gh_panic(@"invalid argument to Int63n");
    return (int64_t)(gh_random() % (uint64_t)n);
}

static int32_t gh_rand_intn(int32_t n) { return gh_rand_int31n(n); }

static uint32_t gh_rand_uint32(void) { return (uint32_t)(gh_random() >> 32); }

static uint64_t gh_rand_uint64(void) { return gh_random(); }

static float gh_rand_float32(void) { return (float)((gh_random() >> 40) * (1.0 / 16777216.0)); }

static double gh_rand_float64(void) { return (double)(gh_random() >> 11) * (1.0 / 9007199254740992.0); }

static void gh_os_exit(int32_t code) { exit(code); }

static NSString *gh_os_getenv(NSString *key) {
    NSDictionary *environment = [[NSProcessInfo processInfo] environment];
    return environment[key] ?: @"";
}

static bool gh_os_setenv(NSString *key, NSString *value) {
    return setenv([key UTF8String], [value UTF8String], 1) == 0;
}

static bool gh_os_unsetenv(NSString *key) { return unsetenv([key UTF8String]) == 0; }

static NSString *gh_os_executable(void) {
    NSArray *arguments = [[NSProcessInfo processInfo] arguments];
    return arguments.count ? [arguments objectAtIndex:0] : @"";
}

static NSString *gh_os_hostname(void) {
    char buffer[256];
    buffer[0] = 0;
    if (gethostname(buffer, sizeof(buffer)) != 0) buffer[0] = 0;
    buffer[sizeof(buffer) - 1] = 0;
    return gh_string(buffer, strlen(buffer));
}

static NSString *gh_os_temp_dir(void) { return NSTemporaryDirectory() ?: @"/tmp"; }

static int32_t gh_os_getpid(void) { return (int32_t)getpid(); }

static int32_t gh_os_getuid(void) { return (int32_t)getuid(); }

static int32_t gh_os_geteuid(void) { return (int32_t)geteuid(); }

static NSString *gh_os_read_file(NSString *name) {
    return [NSString stringWithContentsOfFile:name encoding:NSUTF8StringEncoding error:nil] ?: @"";
}

static bool gh_os_write_file(NSString *name, NSString *contents) {
    return [contents writeToFile:name atomically:YES encoding:NSUTF8StringEncoding error:nil];
}

static bool gh_os_remove(NSString *name) {
    return [[NSFileManager defaultManager] removeItemAtPath:name error:nil];
}

static bool gh_os_remove_all(NSString *path) {
    return [[NSFileManager defaultManager] removeItemAtPath:path error:nil];
}

static bool gh_os_rename(NSString *oldpath, NSString *newpath) {
    return [[NSFileManager defaultManager] moveItemAtPath:oldpath toPath:newpath error:nil];
}

static bool gh_os_mkdir(NSString *name) {
    return [[NSFileManager defaultManager] createDirectoryAtPath:name withIntermediateDirectories:NO attributes:nil error:nil];
}

static bool gh_os_mkdir_all(NSString *path) {
    return [[NSFileManager defaultManager] createDirectoryAtPath:path withIntermediateDirectories:YES attributes:nil error:nil];
}

static volatile int32_t gh_runtime_sink;

static void gh_runtime_gc(void) { gh_runtime_sink++; }

static void gh_runtime_gosched(void) { gh_runtime_sink++; }

static int32_t gh_runtime_num_cpu(void) { return 1; }

static int32_t gh_runtime_num_goroutine(void) { return 1; }

static int32_t gh_runtime_gomaxprocs(int32_t n) { return (n < 1 ? 1 : n); }

static const int64_t GH_NANOSECOND = 1000000000LL;
static const int64_t GH_DAY = 86400LL * GH_NANOSECOND;

static const char *GH_MONTHS[12] = {"January", "February", "March", "April", "May", "June",
                                     "July", "August", "September", "October", "November", "December"};
static const char *GH_WEEKDAYS[7] = {"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"};

static void gh_civil(int64_t days, int32_t *year, int32_t *month, int32_t *day) {
    int64_t z = days + 719468;
    int64_t era = (z >= 0 ? z : z - 146096) / 146097;
    int64_t doe = z - era * 146097;
    int64_t yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
    int64_t y = yoe + era * 400;
    int64_t doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    int64_t mp = (5 * doy + 2) / 153;
    int64_t d = doy - (153 * mp + 2) / 5 + 1;
    int64_t m = mp + (mp < 10 ? 3 : -9);
    *year = (int32_t)(y + (m <= 2));
    *month = (int32_t)m;
    *day = (int32_t)d;
}

static int64_t gh_epoch_day(int32_t year, int32_t month, int32_t day) {
    int64_t y = year - (month <= 2);
    int64_t era = (y >= 0 ? y : y - 399) / 400;
    int64_t yoe = y - era * 400;
    int64_t doy = (153 * (month + (month > 2 ? -3 : 9)) + 2) / 5 + day - 1;
    return era * 146097 + yoe * 365 + yoe / 4 - yoe / 100 + doy - 719468;
}

static int64_t gh_day_of(int64_t t) {
    int64_t days = t / GH_DAY;
    if (t % GH_DAY && t < 0) days--;
    return days;
}

static int64_t gh_second_of_day(int64_t t) { return (t - gh_day_of(t) * GH_DAY) / GH_NANOSECOND; }

static int64_t gh_now(void) {
    double stamp = [[NSDate date] timeIntervalSince1970];
    int64_t seconds = (int64_t)stamp;
    return seconds * GH_NANOSECOND + (int64_t)((stamp - (double)seconds) * 1e9 + 0.5);
}

static int32_t gh_time_hour(int64_t t) { return (int32_t)(gh_second_of_day(t) / 3600); }

static int32_t gh_time_minute(int64_t t) { return (int32_t)(gh_second_of_day(t) / 60 % 60); }

static int32_t gh_time_second(int64_t t) { return (int32_t)(gh_second_of_day(t) % 60); }

static int32_t gh_time_nanosecond(int64_t t) { return (int32_t)(t - gh_day_of(t) * GH_DAY - gh_second_of_day(t) * GH_NANOSECOND); }

static int32_t gh_time_month(int64_t t) {
    int32_t year, month, day;
    gh_civil(gh_day_of(t), &year, &month, &day);
    return month;
}

static int32_t gh_time_day(int64_t t) {
    int32_t year, month, day;
    gh_civil(gh_day_of(t), &year, &month, &day);
    return day;
}

static int32_t gh_time_year(int64_t t) {
    int32_t year, month, day;
    gh_civil(gh_day_of(t), &year, &month, &day);
    return year;
}

static int32_t gh_time_year_day(int64_t t) {
    int64_t days = gh_day_of(t);
    return (int32_t)(days - gh_epoch_day(gh_time_year(t), 1, 1) + 1);
}

static int32_t gh_time_weekday(int64_t t) {
    int64_t weekday = (gh_day_of(t) + 4) % 7;
    return (int32_t)(weekday < 0 ? weekday + 7 : weekday);
}

static bool gh_at(NSString *layout, NSUInteger i, const char *token) {
    for (size_t n = 0; token[n]; n++) {
        if (i + n >= layout.length) return false;
        if ([layout characterAtIndex:i + n] != (unichar)token[n]) return false;
    }
    return true;
}

static NSString *gh_time_layout(int64_t t, NSString *layout) {
    NSMutableString *out = [NSMutableString string];
    if (!layout) return out;
    int32_t year = gh_time_year(t), month = gh_time_month(t), day = gh_time_day(t);
    int32_t hour = gh_time_hour(t), minute = gh_time_minute(t), second = gh_time_second(t);
    int32_t weekday = gh_time_weekday(t);
    NSUInteger length = layout.length;
    for (NSUInteger i = 0; i < length; i++) {
        unichar c = [layout characterAtIndex:i];
        if (gh_at(layout, i, "January")) { [out appendFormat:@"%s", GH_MONTHS[month - 1]]; i += 6; continue; }
        if (gh_at(layout, i, "Monday")) { [out appendFormat:@"%s", GH_WEEKDAYS[weekday]]; i += 5; continue; }
        if (gh_at(layout, i, "2006")) { [out appendFormat:@"%04d", year]; i += 3; continue; }
        if (gh_at(layout, i, "Jan")) { [out appendFormat:@"%.3s", GH_MONTHS[month - 1]]; i += 2; continue; }
        if (gh_at(layout, i, "Mon")) { [out appendFormat:@"%.3s", GH_WEEKDAYS[weekday]]; i += 2; continue; }
        if (gh_at(layout, i, "01")) { [out appendFormat:@"%02d", month]; i++; continue; }
        if (gh_at(layout, i, "15")) { [out appendFormat:@"%02d", hour]; i++; continue; }
        if (gh_at(layout, i, "03")) { [out appendFormat:@"%02d", hour % 12 ? hour % 12 : 12]; i++; continue; }
        if (gh_at(layout, i, "04")) { [out appendFormat:@"%02d", minute]; i++; continue; }
        if (gh_at(layout, i, "05")) { [out appendFormat:@"%02d", second]; i++; continue; }
        if (gh_at(layout, i, "02")) { [out appendFormat:@"%02d", day]; i++; continue; }
        if (gh_at(layout, i, "_2")) { [out appendFormat:@"%@%d", day < 10 ? @" " : @"", day]; i++; continue; }
        if (gh_at(layout, i, "06")) { [out appendFormat:@"%02d", year % 100]; i++; continue; }
        if (gh_at(layout, i, "PM")) { [out appendString:hour < 12 ? @"AM" : @"PM"]; i++; continue; }
        if (gh_at(layout, i, "pm")) { [out appendString:hour < 12 ? @"am" : @"pm"]; i++; continue; }
        if (gh_at(layout, i, "Z07:00")) { [out appendString:@"Z"]; i += 5; continue; }
        if (gh_at(layout, i, "-0700")) { [out appendString:@"+0000"]; i += 4; continue; }
        if (gh_at(layout, i, "Z0700")) { [out appendString:@"+0000"]; i += 4; continue; }
        if (gh_at(layout, i, "-07:00")) { [out appendString:@"+00:00"]; i += 5; continue; }
        if (gh_at(layout, i, "-07")) { [out appendString:@"+00"]; i += 2; continue; }
        if (gh_at(layout, i, "MST")) { [out appendString:@"UTC"]; i += 2; continue; }
        if (c == '.') {
            unichar kind = i + 1 < length ? [layout characterAtIndex:i + 1] : 0;
            if (kind == '0' || kind == '9') {
                NSUInteger count = 0;
                while (i + 1 + count < length && [layout characterAtIndex:i + 1 + count] == kind) count++;
                if (count > 9) count = 9;
                int64_t divisor = 1;
                for (NSUInteger n = count; n < 9; n++) divisor *= 10;
                int64_t value = gh_time_nanosecond(t);
                if (kind == '9') value = (value + divisor / 2) / divisor;
                else value /= divisor;
                NSString *slice = [NSString stringWithFormat:@"%0*lld", (int)count, (long long)value];
                NSUInteger end = slice.length;
                while (end > 0 && [slice characterAtIndex:end - 1] == '0') end--;
                if (end > 0) [out appendFormat:@".%@", [slice substringToIndex:end]];
                i += count;
                continue;
            }
        }
        if (c == '1') { [out appendFormat:@"%d", month]; continue; }
        if (c == '2') { [out appendFormat:@"%d", day]; continue; }
        if (c == '3') { [out appendFormat:@"%d", hour % 12 ? hour % 12 : 12]; continue; }
        if (c == '4') { [out appendFormat:@"%d", minute]; continue; }
        if (c == '5') { [out appendFormat:@"%d", second]; continue; }
        [out appendFormat:@"%C", c];
    }
    return out;
}

static int64_t gh_time_now(void) { return gh_now(); }

static int64_t gh_time_since(int64_t t) { return gh_now() - t; }

static int64_t gh_time_until(int64_t t) { return t - gh_now(); }

static int64_t gh_time_unix(int64_t sec, int64_t nsec) { return sec * GH_NANOSECOND + nsec; }

static void gh_time_sleep(int64_t d) {
    double seconds = (double)d / 1e9;
    if (seconds <= 0) return;
    if (seconds > 2592000) seconds = 2592000;
    [NSThread sleepForTimeInterval:seconds];
}

static int64_t gh_time_date(int32_t year, int32_t month, int32_t day, int32_t hour, int32_t minute, int32_t second, int32_t nanosecond) {
    return gh_epoch_day(year, month, day) * GH_DAY + ((int64_t)hour * 3600 + (int64_t)minute * 60 + second) * GH_NANOSECOND + nanosecond;
}

static int64_t gh_time_add(int64_t t, int64_t d) { return t + d; }

static int64_t gh_time_sub(int64_t t, int64_t u) { return t - u; }

static int64_t gh_time_compare(int64_t t, int64_t u) { return t == u ? 0 : (t < u ? -1 : 1); }

static bool gh_time_before(int64_t t, int64_t u) { return t < u; }

static bool gh_time_after(int64_t t, int64_t u) { return t > u; }

static bool gh_time_equal(int64_t t, int64_t u) { return t == u; }

static bool gh_time_is_zero(int64_t t) { return t == 0; }

static int64_t gh_time_truncate(int64_t t, int64_t d) {
    if (d <= 0) gh_panic(@"non-positive time.Time.Truncate duration");
    int64_t rest = t % d;
    if (rest < 0) rest += d;
    return t - rest;
}

static int64_t gh_time_round(int64_t t, int64_t d) {
    if (d <= 0) gh_panic(@"non-positive time.Time.Round duration");
    int64_t rest = t % d;
    if (rest < 0) rest += d;
    return rest + d / 2 < d ? t - rest : t - rest + d;
}

static int64_t gh_time_unix_nano(int64_t t) { return t; }

static int64_t gh_time_unix_milli(int64_t t) { return t / 1000000; }

static NSString *gh_time_format(int64_t t, NSString *layout) { return gh_time_layout(t, layout); }

static NSString *gh_time_string(int64_t t) { return gh_time_layout(t, @"2006-01-02 15:04:05.999999999 -0700 MST"); }

static double gh_duration_seconds(int64_t d) { return (double)d / 1e9; }

static double gh_duration_minutes(int64_t d) { return (double)d / 6e10; }

static double gh_duration_hours(int64_t d) { return (double)d / 3.6e12; }

static int64_t gh_duration_nanoseconds(int64_t d) { return d; }

static int64_t gh_duration_microseconds(int64_t d) { return d / 1000; }

static int64_t gh_duration_milliseconds(int64_t d) { return d / 1000000; }

static NSString *gh_duration_string(int64_t d) {
    if (!d) return @"0s";
    uint64_t magnitude = d < 0 ? (uint64_t)(-(d + 1)) + 1 : (uint64_t)d;
    NSMutableString *out = [NSMutableString string];
    if (d < 0) [out appendString:@"-"];
    if (magnitude < GH_NANOSECOND) {
        uint64_t divisor = magnitude < 1000 ? 1 : (magnitude < 1000000 ? 1000 : 1000000);
        const char *unit = divisor == 1 ? "ns" : (divisor == 1000 ? "\xc2\xb5s" : "ms");
        uint64_t value = magnitude / divisor, fraction = magnitude % divisor;
        if (!fraction) return [NSString stringWithFormat:@"%@%llu%s", out, (unsigned long long)value, unit];
        NSString *tail = [NSString stringWithFormat:@"%06llu", (unsigned long long)(fraction * 1000000 / divisor)];
        NSUInteger end = tail.length;
        while (end > 0 && [tail characterAtIndex:end - 1] == '0') end--;
        return [NSString stringWithFormat:@"%@%llu.%@%s", out, (unsigned long long)value, [tail substringToIndex:end], unit];
    }
    uint64_t seconds = magnitude / (uint64_t)GH_NANOSECOND, fraction = magnitude % (uint64_t)GH_NANOSECOND;
    uint64_t hours = seconds / 3600, minutes = seconds / 60 % 60;
    if (hours) [out appendFormat:@"%lluh", (unsigned long long)hours];
    if (hours || minutes) [out appendFormat:@"%llum", (unsigned long long)minutes];
    if (!fraction) [out appendFormat:@"%llus", (unsigned long long)(seconds % 60)];
    else {
        NSString *tail = [NSString stringWithFormat:@"%09llu", (unsigned long long)fraction];
        NSUInteger end = tail.length;
        while (end > 0 && [tail characterAtIndex:end - 1] == '0') end--;
        [out appendFormat:@"%llu.%@s", (unsigned long long)(seconds % 60), [tail substringToIndex:end]];
    }
    return out;
}

#if GH_APP
@interface GHAppDelegate : UIResponder <UIApplicationDelegate>
@property(nonatomic, strong) UIWindow *window;
@end

@implementation GHAppDelegate
- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)options {
    self.window = [[UIWindow alloc] initWithFrame:[[UIScreen mainScreen] bounds]];
    self.window.backgroundColor = [UIColor whiteColor];
    UIViewController *plain = [[UIViewController alloc] init];
    gh_root = plain.view;
    gh_root.backgroundColor = [UIColor whiteColor];
    gh_init();
    gh_main();
    UITabBarController *tabs = gh_tab_count ? gh_build_tabs() : nil;
    self.window.rootViewController = tabs ? (UIViewController *)tabs : plain;
    [self.window makeKeyAndVisible];
    if (tabs) gh_show([tabs.viewControllers objectAtIndex:0], &gh_tabs[0]);
    return YES;
}
@end

int main(int argc, char **argv) {
    @autoreleasepool { return UIApplicationMain(argc, argv, nil, NSStringFromClass([GHAppDelegate class])); }
}
#else
__attribute__((constructor)) static void gh_load(void) {
    @autoreleasepool { gh_init(); gh_main(); }
}
#endif
