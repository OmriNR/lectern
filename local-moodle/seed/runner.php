<?php
// Generic seed-data runner. This file is fixed plumbing: it never encodes any
// university-specific data itself. It reads a JSON spec (built by the Go
// program in this directory) describing majors/categories/courses/lectures/
// assignments, and creates them using Moodle's internal generator APIs -
// the only reliable way to create fully-configured activities (Moodle's
// web service layer has no supported "create activity" endpoint).
//
// Usage (inside the moodle container):
//   php /bitnami/moodle/seed_runner.php /bitnami/moodle/seed_spec.json

define('CLI_SCRIPT', true);
require(__DIR__ . '/config.php');
require_once($CFG->dirroot . '/lib/testing/generator/lib.php');
require_once($CFG->dirroot . '/course/lib.php');

global $DB;

$specpath = $argv[1] ?? null;
if (!$specpath || !is_readable($specpath)) {
    fwrite(STDERR, "usage: php seed_runner.php <spec.json>\n");
    exit(1);
}

$spec = json_decode(file_get_contents($specpath), true);
if ($spec === null) {
    fwrite(STDERR, "invalid JSON in spec file\n");
    exit(1);
}

$admin = $DB->get_record('user', ['username' => 'admin'], '*', MUST_EXIST);
\core\session\manager::set_user($admin);

$generator = new testing_data_generator();
$fs = get_file_storage();

foreach ($spec['majors'] as $majorspec) {
    $existing = $DB->get_record('course_categories', ['name' => $majorspec['name'], 'parent' => 0]);
    if ($existing) {
        $major = $existing;
        echo "Major: {$majorspec['name']} (id {$major->id}, already exists)\n";
    } else {
        $major = $generator->create_category(['name' => $majorspec['name']]);
        echo "Major: {$majorspec['name']} (id {$major->id})\n";
    }

    foreach ($majorspec['categories'] as $catspec) {
        $existing = $DB->get_record('course_categories', ['name' => $catspec['name'], 'parent' => $major->id]);
        if ($existing) {
            $category = $existing;
            echo "  Category: {$catspec['name']} (id {$category->id}, already exists)\n";
        } else {
            $category = $generator->create_category([
                'name' => $catspec['name'],
                'parent' => $major->id,
            ]);
            echo "  Category: {$catspec['name']} (id {$category->id})\n";
        }

        foreach ($catspec['courses'] as $coursespec) {
            $existingcourse = $DB->get_record('course', ['shortname' => $coursespec['shortname']]);
            if ($existingcourse) {
                echo "    Course: {$coursespec['fullname']} ({$coursespec['shortname']}, id {$existingcourse->id}, already exists - skipping content)\n";
                continue;
            }

            $course = $generator->create_course([
                'shortname' => $coursespec['shortname'],
                'fullname' => $coursespec['fullname'],
                'summary' => $coursespec['summary'],
                'category' => $category->id,
                'startdate' => $coursespec['startdate'],
                'numsections' => 4,
            ]);
            echo "    Course: {$coursespec['fullname']} ({$coursespec['shortname']}, id {$course->id})\n";

            $generator->enrol_user($admin->id, $course->id, 'student');

            $section = 1;
            foreach ($coursespec['lectures'] as $lecture) {
                $generator->create_module('page', [
                    'course' => $course->id,
                    'name' => $lecture['name'],
                    'content' => $lecture['content'],
                    'section' => $section,
                ]);
                echo "      Page: {$lecture['name']}\n";
                $section++;
            }

            foreach ($coursespec['assignments'] as $assignment) {
                $moduleinfo = $generator->create_module('assign', [
                    'course' => $course->id,
                    'name' => $assignment['name'],
                    'intro' => $assignment['intro'],
                    'duedate' => $assignment['duedate'],
                    'allowsubmissionsfromdate' => $assignment['allowsubmissionsfromdate'],
                    'section' => $section,
                ]);

                $context = context_module::instance($moduleinfo->cmid);
                $filecontent = base64_decode($assignment['filecontentbase64']);
                $fs->create_file_from_string([
                    'component' => 'mod_assign',
                    'filearea' => 'introattachment',
                    'contextid' => $context->id,
                    'itemid' => 0,
                    'filename' => $assignment['filename'],
                    'filepath' => '/',
                ], $filecontent);

                echo "      Assign: {$assignment['name']} (file: {$assignment['filename']})\n";
                $section++;
            }
        }
    }
}

foreach ($spec['users'] as $userspec) {
    $existinguser = $DB->get_record('user', ['username' => $userspec['username'], 'deleted' => 0]);
    if ($existinguser) {
        $user = $existinguser;
        echo "User: {$userspec['username']} ({$userspec['firstname']} {$userspec['lastname']}, id {$user->id}, already exists)\n";
    } else {
        $user = $generator->create_user([
            'username' => $userspec['username'],
            'password' => $userspec['password'],
            'firstname' => $userspec['firstname'],
            'lastname' => $userspec['lastname'],
            'email' => $userspec['email'],
        ]);
        echo "User: {$userspec['username']} ({$userspec['firstname']} {$userspec['lastname']}, id {$user->id})\n";
    }

    foreach ($userspec['courses'] as $shortname) {
        $course = $DB->get_record('course', ['shortname' => $shortname], '*', MUST_EXIST);

        $alreadyenrolled = is_enrolled(context_course::instance($course->id), $user->id);
        if ($alreadyenrolled) {
            echo "  Already enrolled in {$shortname}\n";
            continue;
        }

        // 'student' role only: no moodle/course:update or category:manage capability,
        // so these users cannot edit courses or categories.
        $generator->enrol_user($user->id, $course->id, 'student');
        echo "  Enrolled (student) in {$shortname}\n";
    }
}

echo "\nDone.\n";
